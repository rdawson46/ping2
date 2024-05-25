package ping

// WARN: using ip requires root priv

import (
    "fmt"
    "math/rand"
    "net"
    "sync"
    "syscall"
    "time"
    "golang.org/x/net/icmp"
    "golang.org/x/net/ipv4"
    "golang.org/x/net/ipv6"
)

const (
    TimeSliceLength = 8
    ProtocolICMP = 1
    ProtocolIPv6ICMP = 58
)

var (
    ipv4Proto = map[string]string{"ip": "ip4:icmp", "udp": "udp4"}
    ipv6Proto = map[string]string{"ip": "ip6:ipv6-icmp", "udp": "udp6"}
)

func byteSliceOfSize(n int) []byte {
    b:= make([]byte, n)
    for i := 0; i < len(b); i ++ {
        b[i] = 1
    }

    return b
}

func timeToBytes(t time.Time) []byte {
    nsec := t.UnixNano()
    b := make([]byte, 8)
    for i := uint8(0); i < 8; i++ {
        b[i] = byte((nsec >> ((7 - i) * 8)) & 0xff)
    }
    return b
}

func bytesToTime(b []byte) time.Time {
    var nsec int64
    for i := uint8(0); i < 8; i++ {
        nsec += int64(b[i]) << ((7 - i) * 8)
    }

    return time.Unix(nsec / 1_000_000_000, nsec % 1_000_000_000)
}

func isIPv4(ip net.IP) bool {
    return len(ip.To4()) == net.IPv4len
}

func isIPv6(ip net.IP) bool {
    return len(ip) == net.IPv6len
}

func ipv4Payload(b []byte) []byte {
    if len(b) < ipv4.HeaderLen {
        return b
    }
    hdrlen := int(b[0] & 0x0f) << 2
    return b[hdrlen:]
}

type packet struct {
    bytes []byte
    addr  net.Addr
}

type context struct {
    stop chan bool
    done chan bool
    err  error
}

func newContext() *context {
    return &context{
        stop: make(chan bool),
        done: make(chan bool),
    }
}

// ICMP packet sender/reciever
type Pinger struct {
    id      int
    seq     int
    addrs   map[string]*net.IPAddr
    network string
    source  string
    source6 string
    hasIPv4 bool
    hasIPv6 bool
    ctx     *context
    mu      sync.Mutex
    Size    int
    MaxRTT  time.Duration
    OnRecv  func(*net.IPAddr, time.Duration)
    OnIdle  func()
    Debug   bool
}

func NewPinger() *Pinger {
    gen := rand.New(rand.NewSource(time.Now().UnixNano()))

    return &Pinger{
        id: gen.Intn(0xffff),
        seq: gen.Intn(0xffff),
        addrs: make(map[string]*net.IPAddr),
        network: "ip",
        source: "",
        source6: "",
        hasIPv4: false,
        hasIPv6: false,
        Size: TimeSliceLength,
        MaxRTT: time.Second,
        OnRecv: nil,
        OnIdle: nil,
        Debug: false,
    }
}

func (p *Pinger) AddIPAddr(ip *net.IPAddr) {
    p.mu.Lock()
    p.addrs[ip.String()] = ip
    if isIPv4(ip.IP) {
        p.hasIPv4 = true
    } else if isIPv6(ip.IP) {
        p.hasIPv6 = true
    }
    p.mu.Unlock()
}

func (p *Pinger) AddIP(ipadder string) error {
    addr := net.ParseIP(ipadder)
    if addr == nil {
        return fmt.Errorf("%s is not valid", ipadder)
    }

    p.mu.Lock()
    p.addrs[addr.String()] = &net.IPAddr{IP: addr}
    if isIPv4(addr) {
        p.hasIPv4 = true
    } else if isIPv6(addr) {
        p.hasIPv6 = true
    }
    p.mu.Unlock()
    return nil
}

func (p *Pinger) RemoveIP(ipadder string) error {
    addr := net.ParseIP(ipadder)
    if addr == nil {
        return fmt.Errorf("%s not valid", ipadder)
    }
    p.mu.Lock()
    delete(p.addrs, addr.String())
    p.mu.Unlock()
    return nil
}

func (p *Pinger) Run() error {
    p.mu.Lock()
    p.ctx = newContext()
    p.mu.Unlock()
    p.run(true)
    p.mu.Lock()
    defer p.mu.Unlock()
    return p.ctx.err
}

func (p *Pinger) RunLoop() {
    p.mu.Lock()
    p.ctx = newContext()
    p.mu.Unlock()
    go p.run(false)
}

func (p *Pinger) Done() <-chan bool {
    return p.ctx.done
}

func (p *Pinger) Stop() {
    close(p.ctx.stop)
    <- p.ctx.done
}

func (p *Pinger) Err() error {
    p.mu.Lock()
    defer p.mu.Unlock()
    return p.ctx.err
}

func (p *Pinger) listen(netProto, source string) *icmp.PacketConn {
    conn, err := icmp.ListenPacket(netProto, source)
    if err != nil {
        p.mu.Lock()
        p.ctx.err = err
        p.mu.Unlock()
        close(p.ctx.done)
        return nil
    }

    return conn
}

func (p *Pinger) run(once bool) {
    var conn, conn6 *icmp.PacketConn
    if p.hasIPv4 {
        if conn = p.listen(ipv4Proto[p.network], p.source); conn == nil {
            return
        }
        defer conn.Close()
    } else if p.hasIPv6 {
        if conn6 = p.listen(ipv6Proto[p.network], p.source6); conn6 == nil {
            return
        }
        defer conn.Close()
    }

    recv := make(chan *packet, 1)
    recvCtx := newContext()
    wg := new(sync.WaitGroup)

    if conn != nil {
        wg.Add(1)
        go p.recvICMP(conn, recv, recvCtx, wg)
    }
    if conn6 != nil {
        wg.Add(1)
        go p.recvICMP(conn6, recv, recvCtx, wg)
    }

    queue, err := p.sendICMP(conn, conn6)
    ticker := time.NewTicker(p.MaxRTT)

mainloop:
        for {
            select {
            case <-p.ctx.stop:
                break mainloop
            case <- recvCtx.done:
                p.mu.Lock()
                err = recvCtx.err
                p.mu.Unlock()
                break mainloop
            case <- ticker.C:
                p.mu.Lock()
                handler := p.OnIdle
                p.mu.Unlock()
                if handler != nil {
                    handler()
                }
                if once || err != nil {
                    break mainloop
                }
                queue, err = p.sendICMP(conn, conn6)
            case r := <-recv:
                p.procRecv(r, queue)
            }
        }

        ticker.Stop()

        close(recvCtx.stop)
        wg.Wait()

        p.mu.Lock()
        p.ctx.err = err
        p.mu.Unlock()

        close(p.ctx.done)
}

func (p *Pinger) sendICMP(conn, conn6 *icmp.PacketConn) (map[string]*net.IPAddr, error) {
    p.mu.Lock()
    p.id = rand.Intn(0xffff)
    p.seq = rand.Intn(0xffff)
    p.mu.Unlock()
    queue := make(map[string]*net.IPAddr)
    wg := new(sync.WaitGroup)
    for key, addr := range p.addrs {
        var typ icmp.Type
        var cn *icmp.PacketConn
        if isIPv4(addr.IP) {
            typ = ipv4.ICMPTypeEcho
            cn = conn
        } else if isIPv6(addr.IP) {
            typ = ipv6.ICMPTypeEchoRequest
            cn = conn6
        } else {
            continue
        }
        if cn == nil {
            continue
        }

        t := timeToBytes(time.Now())
        
        if p.Size - TimeSliceLength != 0 {
            t = append(t, byteSliceOfSize(p.Size - TimeSliceLength)...)
        }

        p.mu.Lock()
        bytes, err := (&icmp.Message{
            Type: typ,
            Code: 0,
            Body: &icmp.Echo{
                ID: p.id,
                Seq: p.seq,
                Data: t,
            },
        }).Marshal(nil)
        p.mu.Unlock()
        if err != nil {
            wg.Wait()
            return queue, err
        }

        queue[key] = addr
        var dst net.Addr = addr
        if p.network == "udp" {
            dst = &net.UDPAddr{IP: addr.IP, Zone: addr.Zone}
        }

        wg.Add(1)
        go func(conn * icmp.PacketConn, ra net.Addr, b []byte) {
            for {
                if _, err := conn.WriteTo(bytes, ra); err != nil {
                    if neterr, ok := err.(*net.OpError); ok {
                        if neterr.Err == syscall.ENOBUFS {
                            continue
                        }
                    }
                }
                break
            }

            wg.Done()
        }(cn, dst, bytes)
    }

    wg.Wait()
    return queue, nil
}

func (p *Pinger) recvICMP(conn *icmp.PacketConn, recv chan<- *packet, ctx *context, wg *sync.WaitGroup) {
    for {
        select {
        case <- ctx.stop:
            wg.Done()
            return
        default:
        }

        bytes := make([]byte, 1024)
        conn.SetReadDeadline(time.Now().Add(time.Millisecond * 100))
        _, ra, err := conn.ReadFrom(bytes)

        if err != nil {
            if neterr, ok := err.(*net.OpError); ok {
                if neterr.Timeout() {
                    continue
                } else {
                    p.mu.Lock()
                    ctx.err = err
                    p.mu.Unlock()
                    wg.Done()
                    return
                }
            }
        }

        select {
        case recv <- &packet{bytes: bytes, addr: ra}:
        case <- ctx.stop:
            wg.Done()
            return
        }
    }
}

func (p *Pinger) procRecv(recv *packet, queue map[string]*net.IPAddr) {
    var ipaddr *net.IPAddr
    switch adr := recv.addr.(type) {
    case *net.IPAddr:
        ipaddr = adr
    case *net.UDPAddr:
        ipaddr = &net.IPAddr{IP: adr.IP, Zone: adr.Zone}
    default:
        return
    }

    addr := ipaddr.String()
    p.mu.Lock()
    if _, ok := p.addrs[addr]; !ok {
        p.mu.Unlock()
        return
    }
    p.mu.Unlock()

    var bytes []byte
    var proto int
    if isIPv4(ipaddr.IP) {
        if p.network == "ip" {
            bytes = ipv4Payload(recv.bytes)
        } else {
            bytes = recv.bytes
        }
        proto = ProtocolICMP
    } else if isIPv6(ipaddr.IP) {
        bytes = recv.bytes
        proto = ProtocolIPv6ICMP
    } else {
        return
    }

    var m *icmp.Message
    var err error
    if m, err = icmp.ParseMessage(proto, bytes); err != nil {
        return
    }

    if m.Type != ipv4.ICMPTypeEchoReply && m.Type != ipv6.ICMPTypeEchoReply {
        return
    }

    var rtt time.Duration
    switch pkt := m.Body.(type) {
    case *icmp.Echo:
        p.mu.Lock()
        if pkt.ID == p.id && pkt.Seq == p.seq {
            rtt = time.Since(bytesToTime(pkt.Data[:TimeSliceLength]))
        }
        p.mu.Unlock()
    default:
        return
    }

    if _, ok := queue[addr]; ok {
        delete(queue, addr)
        p.mu.Lock()
        handler := p.OnRecv
        p.mu.Unlock()
        if handler != nil {
            handler(ipaddr, rtt)
        }
    }
}
