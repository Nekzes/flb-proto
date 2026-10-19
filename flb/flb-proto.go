package flb

import (
	"encoding/binary"
	"errors"
	"net"
	"sync/atomic"
	"time"
)

const (
	PrPing uint8 = 1
	PrPong uint8 = 2
	PrEcho uint8 = 3
)

const (
	StPrDone  uint8 = 1
	StPrError uint8 = 0
)

const (
	Version       uint8 = 2
	MaxBufferSize       = 2048
	MaxAttemp           = 10
	HeaderSize          = 8
	Timeout             = 300 * time.Millisecond
)

const (
	FlagNone    uint8 = 0      //00000000
	FlagOneWay  uint8 = 1 << 0 //00000001
	FlagNeedACK uint8 = 1 << 1 //00000010
	FlagIsACK   uint8 = 1 << 2 //00000100
)

func NewPackage(data_bytes []byte, prefix_data uint8, stat_prefix uint8, flags uint8, seq uint32) *Package {

	return &Package{
		Version:       Version,
		Prefix:        prefix_data,
		Status_prefix: stat_prefix,
		Flags:         flags,
		Seq:           seq,
		Data:          data_bytes,
	}

}

type Package struct {
	Version       uint8
	Prefix        uint8
	Status_prefix uint8
	Flags         uint8
	Seq           uint32
	Data          []byte
}

func (pkg *Package) Pack() []byte {
	buf := make([]byte, 0, HeaderSize+len(pkg.Data))
	buf = append(buf, pkg.Version, pkg.Prefix, pkg.Status_prefix, pkg.Flags)
	buf = binary.BigEndian.AppendUint32(buf, pkg.Seq)
	buf = append(buf, pkg.Data...)
	return buf

}

func UnPack(d []byte) (*Package, error) {
	if len(d) < HeaderSize {
		return nil, errors.New("Package too short")
	}
	headers := d[:HeaderSize]
	vers := headers[0]
	prefix := headers[1]
	st_prefix := headers[2]
	flags := headers[3]
	seq := headers[4:8]
	data := d[HeaderSize:]

	return &Package{
		Version:       vers,
		Prefix:        prefix,
		Status_prefix: st_prefix,
		Flags:         flags,
		Seq:           binary.BigEndian.Uint32(seq),
		Data:          data,
	}, nil
}

type Client struct {
	Connect net.Conn
	Buf     []byte
	Seq     atomic.Uint32
}

func NewClient(conn net.Conn) *Client {
	return &Client{
		Connect: conn,
		Buf:     make([]byte, MaxBufferSize),
	}
}
func (c *Client) Close() error {
	err := c.Connect.Close()
	if err != nil {
		return err
	}
	return nil
}
func (c *Client) Read() (*Package, error) {
	n, err := c.Connect.Read(c.Buf)
	if err != nil {
		return nil, err
	}
	pkg, err2 := UnPack(c.Buf[:n])
	if err2 != nil {
		return nil, err2
	}
	return pkg, nil
}

func (c *Client) Write(d []byte) error {
	_, err := c.Connect.Write(d)
	if err != nil {
		return err
	}
	return nil
}

func (c *Client) WriteACK(d []byte) error {
	pkg, err3 := UnPack(d)
	if err3 != nil {
		return errors.New("Data is broken")
	}
	pkg.Flags |= FlagNeedACK
	pkg.Seq = c.Seq.Add(1)
	for attemp := 0; attemp < MaxAttemp; attemp++ {
		err := c.Write(pkg.Pack())
		if err != nil {
			return err
		}
		_ = c.Connect.SetReadDeadline(time.Now().Add(Timeout))
		for {
			resp, err := c.Read()
			if err != nil {
				break
			}
			if (resp.Flags&FlagIsACK) != 0 && resp.Seq == pkg.Seq {
				_ = c.Connect.SetReadDeadline(time.Time{})
				return nil
			}
		}
	}
	_ = c.Connect.SetReadDeadline(time.Time{})
	return errors.New("ACK failed")
}

type HandlerFunc func(pkg *Package, addr net.Addr) *Package

type Server struct {
	Connect  *net.UDPConn
	Buf      []byte
	Handlers map[uint8]HandlerFunc
}

func NewServer(conn *net.UDPConn) *Server {
	return &Server{
		Connect:  conn,
		Buf:      make([]byte, MaxBufferSize),
		Handlers: make(map[uint8]HandlerFunc),
	}
}

func (s *Server) Handle(prefix uint8, h_func HandlerFunc) {
	s.Handlers[prefix] = h_func
}

func (s *Server) Close() error {
	err := s.Connect.Close()
	if err != nil {
		return err
	}
	return nil
}

func (s *Server) WriteTo(d []byte, addr net.Addr) error {
	_, err := s.Connect.WriteTo(d, addr)
	if err != nil {
		return err
	}
	return nil
}

func (s *Server) Read() ([]byte, net.Addr, error) {
	n, addr, err := s.Connect.ReadFrom(s.Buf)
	if err != nil {
		return nil, nil, err
	}
	out := make([]byte, n)
	copy(out, s.Buf[:n])
	return out, addr, nil
}

func (s *Server) ListenAndServe() error {
	for {
		d, addr, err := s.Read()
		if err != nil {
			return err
		}
		pkg, err2 := UnPack(d)
		if err2 != nil {
			continue
		}
		if (pkg.Flags & FlagNeedACK) != 0 {
			ackPkg := NewPackage(nil, pkg.Prefix, StPrDone, FlagIsACK, pkg.Seq)
			_ = s.WriteTo(ackPkg.Pack(), addr)
		}

		handl, ok := s.Handlers[pkg.Prefix]
		if !ok {
			continue
		}
		respPkg := handl(pkg, addr)
		if respPkg == nil {
			continue
		}
		if (pkg.Flags & FlagOneWay) != 0 {
			continue
		}
		s.WriteTo(respPkg.Pack(), addr)

	}
}
