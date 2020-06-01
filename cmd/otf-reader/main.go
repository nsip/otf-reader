package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	otfr "github.com/nsip/otf-reader"
	"github.com/peterbourgon/ff"
)

func main() {

	fs := flag.NewFlagSet("otf-reader", flag.ExitOnError)
	var (
		readerName   = fs.String("name", "", "name for this reader")
		readerID     = fs.String("id", "", "id for this reader, leave blank to auto-generate a unique id")
		providerName = fs.String("provider", "", "name of product or system supplying the data")
		inputFormat  = fs.String("inputFormat", "csv", "format of input data, one of csv|json|xml")
		alignMethod  = fs.String("alignMethod", "", "method to align input data to NLPs must be one of prescribed|mapped|inferred")
		levelMethod  = fs.String("levelMethod", "", "method to apply common scaling this data, one of prescribed|mapped-scale|rules")
		natsPort     = fs.Int("natsPort", 4222, "connection port for nats broker")
		natsHost     = fs.String("natsHost", "localhost", "hostname/ip of nats broker")
		natsCluster  = fs.String("natsCluster", "test-cluster", "cluster id for nats broker")
		topic        = fs.String("topic", "", "nats topic name to publish parsed data items to")
		folder       = fs.String("folder", ".", "folder to watch for data files")
		fileSuffix   = fs.String("suffix", "", "filter files to read by file extension, eg. .csv or .myapp (actual data handling will be determined by input format flag)")
		_            = fs.String("config", "", "config file (optional), json format.")
	)

	ff.Parse(fs, os.Args[1:],
		ff.WithConfigFileFlag("config"),
		ff.WithConfigFileParser(ff.JSONParser),
	)

	opts := []otfr.Option{
		otfr.Name(*readerName),
		otfr.ID(*readerID),
		otfr.ProviderName(*providerName),
		otfr.InputFormat(*inputFormat),
		otfr.LevelMethod(*levelMethod),
		otfr.AlignMethod(*alignMethod),
		otfr.NatsPort(*natsPort),
		otfr.NatsHostName(*natsHost),
		otfr.NatsClusterName(*natsCluster),
		otfr.TopicName(*topic),
		otfr.WatchFolder(*folder),
		otfr.WatchFileSuffix(*fileSuffix),
	}

	rdr, err := otfr.New(opts...)
	if err != nil {
		fmt.Printf("\nCannot create otf-reader:\n%s\n\n", err)
		return
	}

	log.Printf("\n%+v\n", rdr)

}
