package otfreader

import (
	"fmt"
	"log"
	"time"

	"github.com/nsip/otf-reader/internal/util"
	"github.com/radovskyb/watcher"
)

type OTFReader struct {
	name            string
	ID              string
	providerName    string
	inputFormat     string
	levelMethod     string
	alignMethod     string
	natsPort        int
	natsHost        string
	natsCluster     string
	publishTopic    string
	watchFolder     string
	watchFileSuffix string
	interval        time.Duration
	recursive       bool
	dotfiles        bool
	ignore          string
	watcher         *watcher.Watcher
}

//
// create a new reader
//
func New(options ...Option) (*OTFReader, error) {

	rdr := OTFReader{}

	if err := rdr.setOptions(options...); err != nil {
		return nil, err
	}

	return &rdr, nil
}

//
// ensure graceful shutdown of file-watcher
//
func (rdr *OTFReader) Close() {
	rdr.watcher.Close()
}

//
// starts the reader monitoring the filesystem to
// harvest data files.
//
func (rdr *OTFReader) StartWatcher() error {

	// get a nats connection
	sc, err := util.NewConnection(rdr.natsHost, rdr.natsCluster, rdr.name, rdr.natsPort)
	if err != nil {
		return err
	}
	// fmt.Printf("\n\nnats connection:\n\n%+v\n\n", sc)
	_ = sc

	// main watcher event processing loop
	go func() {
		for {
			timer := time.NewTimer(5 * time.Second)
			copying := false
			select {
			case event := <-rdr.watcher.Event:
				if event.Op == watcher.Remove {
					fmt.Println("Delete was fired")
				} else if (event.Op == watcher.Write || event.Op == watcher.Create) && event.IsDir() == false {
					fmt.Println(event)
					copying = true
				}
			case err := <-rdr.watcher.Error:
				log.Fatalln(err)
			case <-rdr.watcher.Closed:
				return
			case <-timer.C:
				if copying {
					fmt.Println("Done with Copy")
					copying = false
				}
			}
			timer.Stop()
		}
	}()

	// Start the watching process.
	if err := rdr.watcher.Start(rdr.interval); err != nil {
		return err
	}

	return nil
}

// filepath | nats
func publishCSV() {

	// remember async publish

}

// filepath | nats
func publishJSON() {

	// remember async publish
}

//
// print the current config
//
func (rdr *OTFReader) PrintConfig() {

	fmt.Println("\n\tOTF-Reader Configuration")
	fmt.Println("\t------------------------\n")

	rdr.printID()
	rdr.printDataConfig()
	rdr.printNatsConfig()
	rdr.printWatcherConfig()

}

func (rdr *OTFReader) printID() {
	fmt.Println("\treader name:\t\t", rdr.name)
	fmt.Println("\treader ID:\t\t", rdr.ID)
}

func (rdr *OTFReader) printDataConfig() {
	fmt.Println("\tdata provider:\t\t", rdr.providerName)
	fmt.Println("\tinput format:\t\t", rdr.inputFormat)
	fmt.Println("\talign method:\t\t", rdr.alignMethod)
	fmt.Println("\tlevel method:\t\t", rdr.levelMethod)
}

func (rdr *OTFReader) printNatsConfig() {
	fmt.Println("\tnats port:\t\t", rdr.natsPort)
	fmt.Println("\tnats host:\t\t", rdr.natsHost)
	fmt.Println("\tnats cluster-id:\t", rdr.natsCluster)
	fmt.Println("\tnats topic:\t\t", rdr.publishTopic)
}

func (rdr *OTFReader) printWatcherConfig() {
	fmt.Println("\twatch file suffix:\t", rdr.watchFileSuffix)
	fmt.Println("\twatch poll interval:\t", rdr.interval)
	fmt.Println("\twatch dot files:\t", rdr.dotfiles)
	fmt.Println("\tignore files:\t\t", rdr.ignore)
	fmt.Println("\twatch folder:\t\t", rdr.watchFolder)
	fmt.Println("\tfiles being watched:")
	for path, f := range rdr.watcher.WatchedFiles() {
		// fmt.Printf("\t   %s: %s\n", path, f.Name())
		_ = path
		fmt.Printf("\t\t\t%s\n", f.Name())
	}
	fmt.Println()

}
