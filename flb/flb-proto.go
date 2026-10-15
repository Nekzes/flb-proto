package flb

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
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

type Prefix struct {
	Is_def    bool
	Stat_answ Status_Prefix
	Pref_answ uint8
	Data_answ string
}

type Status_Prefix struct {
	Dis  string
	Code uint8
}

var Status_prefix_list = map[uint8]Status_Prefix{
	StPrError: {
		Dis:  "Unknow error",
		Code: 0,
	},

	StPrDone: {
		Dis:  "All Done",
		Code: 1,
	},
}
var Prefix_list = map[uint8]Prefix{
	PrPing: {
		Is_def:    false,
		Stat_answ: Status_prefix_list[StPrDone],
		Pref_answ: 2,
		Data_answ: "PONG",
	},
	PrPong: {
		Is_def:    false,
		Stat_answ: Status_prefix_list[StPrDone],
		Pref_answ: 0,
		Data_answ: "",
	},
	PrEcho: {
		Is_def:    true,
		Stat_answ: Status_prefix_list[StPrDone],
		Pref_answ: 0,
	},
}

const version_proto = 1

type Package struct {
	Version       uint8
	Prefix        uint8
	Status_prefix uint8
	Len           uint64
	Data          []byte
}

func NewMessage(data_bytes []byte, prefix_data uint8, stat_prefix uint8) *Package {

	return &Package{
		Data:          data_bytes,
		Prefix:        prefix_data,
		Status_prefix: stat_prefix,
		Len:           uint64(len(data_bytes)),
		Version:       version_proto,
	}
}

func (p *Package) WriteData(wr io.Writer) error {
	package_data := make([]byte, 0, int(11+p.Len))
	package_data = append(package_data, p.Version, p.Prefix, p.Status_prefix)
	package_data = binary.BigEndian.AppendUint64(package_data, p.Len)
	package_data = append(package_data, p.Data...)
	_, err := wr.Write(package_data)
	if err != nil {
		return err
	}
	return nil
}

func ReadData(re io.Reader) ([]byte, error) {
	header_buf := make([]byte, 11)
	_, err := io.ReadFull(re, header_buf)
	if err != nil {
		return nil, err
	}
	vers := header_buf[0]
	prefix := header_buf[1]
	stat_pref := header_buf[2]
	lens := binary.BigEndian.Uint64(header_buf[3:11])

	fmt.Println("Version: ", vers)
	fmt.Println("Prefix: ", prefix)
	fmt.Println("Status Prefix: ", stat_pref)
	fmt.Println("Len: ", lens)
	if lens >= 10*1024*1024 {
		return nil, errors.New("The package is too large")
	}
	data_buf := make([]byte, int(lens))
	_, errs := io.ReadFull(re, data_buf)
	if errs != nil {
		return nil, errs
	}
	fmt.Println("Data: ", string(data_buf))
	return data_buf, nil

}

func HendlPackage(re io.Reader) (*Package, bool, error) {
	header_buf := make([]byte, 11)
	_, err := io.ReadFull(re, header_buf)
	if err != nil {
		return nil, false, err
	}
	vers := header_buf[0]
	prefix := header_buf[1]
	stat_pref := header_buf[2]
	lens := binary.BigEndian.Uint64(header_buf[3:11])

	fmt.Println("Version: ", vers)
	fmt.Println("Prefix: ", prefix)
	fmt.Println("Status Prefix: ", stat_pref)
	fmt.Println("Len: ", lens)

	if lens >= 10*1024*1024 {
		return nil, false, errors.New("The package is too large")
	}

	buf := make([]byte, int(lens))
	_, errs := io.ReadFull(re, buf)
	if errs != nil {
		return nil, false, errs
	}

	if value, ok := Prefix_list[prefix]; ok {
		if value.Is_def != true {
			if value.Pref_answ != 0 {
				return &Package{
					Data:          []byte(value.Data_answ),
					Prefix:        value.Pref_answ,
					Status_prefix: value.Stat_answ.Code,
					Len:           uint64(len(value.Data_answ)),
					Version:       version_proto,
				}, true, nil
			} else {
				return nil, false, nil
			}
		}

		return &Package{
			Version:       version_proto,
			Prefix:        prefix,
			Status_prefix: stat_pref,
			Len:           lens,
			Data:          buf,
		}, false, nil
	} else {
		return nil, false, errors.New("Unknow Prefix")
	}

}
