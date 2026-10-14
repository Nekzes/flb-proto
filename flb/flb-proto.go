package flb

import (
	"encoding/binary"
	"fmt"
	"io"
)

const version_proto = 1

type Package struct {
	data    []byte
	prefix  uint8
	len     uint64
	version uint8
}

func NewMessage(data_bytes []byte, prefix_data uint8) *Package {

	return &Package{
		data:    data_bytes,
		prefix:  prefix_data,
		len:     uint64(len(data_bytes)),
		version: version_proto,
	}
}

func (p *Package) WriteData(wr io.Writer) error {
	package_data := make([]byte, 0, int(10+p.len))
	package_data = append(package_data, p.version, p.prefix)
	package_data = binary.BigEndian.AppendUint64(package_data, p.len)
	package_data = append(package_data, p.data...)
	_, err := wr.Write(package_data)
	if err != nil {
		return err
	}
	return nil
}

func ReadData(re io.Reader) ([]byte, error) {
	header_buf := make([]byte, 10)
	_, err := io.ReadFull(re, header_buf)
	if err != nil {
		return nil, err
	}
	vers := header_buf[0]
	prefix := header_buf[1]
	lens := binary.BigEndian.Uint64(header_buf[2:10])
	fmt.Println("Version: ", vers)
	fmt.Println("Prefix: ", prefix)
	fmt.Println("Len: ", lens)
	data_buf := make([]byte, int(lens))
	_, errs := io.ReadFull(re, data_buf)
	if errs != nil {
		return nil, errs
	}
	fmt.Println("Data: ", string(data_buf))
	return data_buf, nil

}
