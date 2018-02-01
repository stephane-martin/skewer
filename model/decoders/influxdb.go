package decoders

import (
	"strconv"
	"time"

	"github.com/influxdata/influxdb/models"
	"github.com/stephane-martin/skewer/model"
	"golang.org/x/text/encoding"
	"golang.org/x/text/encoding/unicode"
)

func pInflux(m []byte, decoder *encoding.Decoder) (msg *model.SyslogMessage, rerr error) {
	// we assume influxdb line protocol is always UTF-8
	decoder = unicode.UTF8.NewDecoder()

	var err error
	m, err = decoder.Bytes(m)
	if err != nil {
		return nil, &InvalidEncodingError{Err: err}
	}
	points, err := models.ParsePoints(m)
	if err != nil {
		return nil, &InfluxDecodingError{Err: err}
	}
	if len(points) == 0 {
		return nil, nil
	}
	point := points[0]
	msg = model.CleanFactory()
	msg.AppName = "influxdb"
	msg.TimeReportedNum = point.UnixNano()
	msg.TimeGeneratedNum = time.Now().UnixNano()
	msg.Facility = 16
	msg.Severity = 6
	msg.Version = 1
	msg.Message = string(point.Name())
	msg.ProcId = strconv.FormatUint(point.HashID(), 10)
	msg.SetPriority()
	for _, tag := range point.Tags() {
		key := string(tag.Key)
		value := string(tag.Value)
		if key == "host" {
			msg.HostName = value
		}
		msg.SetProperty("influxdb_tags", key, value)
	}
	iter := point.FieldIterator()

Loop:
	for iter.Next() {
		key := string(iter.FieldKey())
		value := ""
		switch iter.Type() {
		case models.Integer:
			val, err := iter.IntegerValue()
			if err != nil {
				continue Loop
			}
			key = key + "_integer"
			value = strconv.FormatInt(val, 10)
		case models.Float:
			val, err := iter.FloatValue()
			if err != nil {
				continue Loop
			}
			key = key + "_float"
			value = strconv.FormatFloat(val, 'f', -1, 64)

		case models.Boolean:
			val, err := iter.BooleanValue()
			if err != nil {
				continue Loop
			}
			key = key + "_boolean"
			value = strconv.FormatBool(val)
		case models.String:
			value = iter.StringValue()
			key = key + "_string"
		case models.Empty:
			continue Loop
			/*
				case models.Unsigned:
					val, err := iter.UnsignedValue()
					if err != nil {
						continue Loop
					}
					key = key + "_unsigned"
					value = strconv.FormatUint(val, 10)
			*/
		}
		msg.SetProperty("influxdb_fields", key, value)
	}
	return msg, nil

}
