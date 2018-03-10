package decoders

import (
	"strconv"
	"sync"
	"time"

	"collectd.org/api"
	"collectd.org/network"
	"github.com/rakyll/statik/fs"
	"github.com/stephane-martin/skewer/model"
	// import the statik store
	_ "github.com/stephane-martin/skewer/utils/collectd/embed/statik"
)

var typesDB *api.TypesDB
var once sync.Once

func pCollectd(m []byte) (msgs []*model.SyslogMessage, rerr error) {
	once.Do(func() {
		statikFS, err := fs.New()
		if err != nil {
			panic(err)
		}
		typesReader, err := statikFS.Open("/types.db")
		if err != nil {
			panic(err)
		}
		typesDB, err = api.NewTypesDB(typesReader)
		if err != nil {
			panic(err)
		}
	})
	opts := network.ParseOpts{
		TypesDB: typesDB,
	}
	lists, err := network.Parse(m, opts)
	if err != nil {
		return nil, err
	}
	var list *api.ValueList
	var values []api.Value
	var value api.Value
	var name string
	var j int
	var gauge api.Gauge
	var derive api.Derive
	var ok bool
	var msg *model.SyslogMessage
	msgs = make([]*model.SyslogMessage, 10)
	var b []byte

	for _, list = range lists {
		msg = model.CleanFactory()
		b, err = list.MarshalJSON()
		if err != nil {
			continue
		}
		msg.Message = string(b)
		msg.TimeReportedNum = list.Time.UnixNano()
		msg.TimeGeneratedNum = time.Now().UnixNano()
		msg.Severity = model.Sinfo
		msg.Facility = model.Fuser
		msg.SetPriority()
		msg.HostName = list.Identifier.Host
		msg.AppName = "collectd"
		msg.MsgId = ""
		msg.ProcId = list.Identifier.Plugin
		msg.Structured = ""
		msg.Version = 1
		msg.ClearDomain("collectd")
		if len(list.Identifier.PluginInstance) > 0 {
			msg.SetProperty("collectd", "plugin_instance", list.Identifier.PluginInstance)
		}
		if len(list.Identifier.Type) > 0 {
			msg.SetProperty("collectd", "type", list.Identifier.Type)
		}
		if len(list.Identifier.TypeInstance) > 0 {
			msg.SetProperty("collectd", "type_instance", list.Identifier.TypeInstance)
		}
		values = list.Values
		for j = range values {
			value = values[j]
			name = list.DSName(j)
			if gauge, ok = value.(api.Gauge); ok {
				msg.SetProperty("gauge", name, strconv.FormatFloat(float64(gauge), 'f', 3, 64))
			} else if derive, ok = value.(api.Derive); ok {
				msg.SetProperty("derive", name, strconv.FormatInt(int64(derive), 10))
			}
		}
		msgs = append(msgs, msg)
	}

	return msgs, nil

}
