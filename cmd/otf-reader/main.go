package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"

	otfr "github.com/nsip/otf-reader"
	"github.com/peterbourgon/ff"
)

func main() {

	fs := flag.NewFlagSet("otf-reader", flag.ExitOnError)
	var (
		readerName    = fs.String("name", "", "name for this reader")
		readerID      = fs.String("id", "", "id for this reader, leave blank to auto-generate a unique id")
		providerName  = fs.String("provider", "", "name of product or system supplying the data")
		inputFormat   = fs.String("inputFormat", "csv", "format of input data, one of csv|json")
		alignMethod   = fs.String("alignMethod", "", "method to align input data to NLPs must be one of prescribed|mapped|inferred")
		levelMethod   = fs.String("levelMethod", "", "method to apply common scaling this data, one of prescribed|mapped-scale|rules")
		genCapability = fs.String("capability", "", "General Capability for assessment results; Literacy or Numeracy")
		natsPort      = fs.Int("natsPort", 4222, "connection port for nats broker")
		natsHost      = fs.String("natsHost", "localhost", "hostname/ip of nats broker")
		natsCluster   = fs.String("natsCluster", "test-cluster", "cluster id for nats broker")
		topic         = fs.String("topic", "", "nats topic name to publish parsed data items to")
		_             = fs.String("config", "", "config file (optional), json format.")
		folder        = fs.String("folder", ".", "folder to watch for data files")
		fileSuffix    = fs.String("suffix", "", "filter files to read by file extension, eg. .csv or .myapp (actual data handling will be determined by input format flag)")
		interval      = fs.String("interval", "500ms", "watcher poll interval")
		recursive     = fs.Bool("recursive", true, "watch folders recursively")
		dotfiles      = fs.Bool("dotfiles", false, "watch dot files")
		ignore        = fs.String("ignore", "", "comma separated list of paths to ignore")
		concurrFiles  = fs.Int("concurrFiles", 10, "pool size for concurrent file processing")
	)

	ff.Parse(fs, os.Args[1:],
		ff.WithConfigFileFlag("config"),
		ff.WithConfigFileParser(ff.JSONParser),
		ff.WithEnvVarPrefix("OTF_RDR"),
	)

	opts := []otfr.Option{
		otfr.Name(*readerName),
		otfr.ID(*readerID),
		otfr.ProviderName(*providerName),
		otfr.InputFormat(*inputFormat),
		otfr.LevelMethod(*levelMethod),
		otfr.AlignMethod(*alignMethod),
		otfr.Capability(*genCapability),
		otfr.NatsPort(*natsPort),
		otfr.NatsHostName(*natsHost),
		otfr.NatsClusterName(*natsCluster),
		otfr.TopicName(*topic),
		otfr.Watcher(*folder, *fileSuffix, *interval, *recursive, *dotfiles, *ignore),
		otfr.ConcurrentFiles(*concurrFiles),
	}

	rdr, err := otfr.New(opts...)
	if err != nil {
		fmt.Printf("\nCannot create otf-reader:\n%s\n\n", err)
		return
	}

	rdr.PrintConfig()

	// signal handler for shutdown
	closed := make(chan struct{})
	c := make(chan os.Signal)
	signal.Notify(c, os.Kill, os.Interrupt)
	go func() {
		<-c
		fmt.Println("\nreader shutting down")
		rdr.Close()
		fmt.Println("otf-reader closed")
		close(closed)
	}()

	// [need to invoke pre-process for existing files/maybe]

	// start the filewatcher
	launchErr := rdr.StartWatcher()
	if launchErr != nil {
		fmt.Printf("\n  Error: Unable to start file watcher: %s\n\n", launchErr)
		close(closed)
	}

	<-closed

}
