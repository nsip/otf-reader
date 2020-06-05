package otfreader

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"strings"
	"time"

	"github.com/cdutwhu/n3-util/n3csv"
	stan "github.com/nats-io/stan.go"
	"github.com/nsip/otf-reader/internal/util"
	"github.com/pkg/errors"
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
	sc              stan.Conn
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
	var connErr error
	rdr.sc, connErr = util.NewConnection(rdr.natsHost, rdr.natsCluster, rdr.name, rdr.natsPort)
	if connErr != nil {
		return connErr
	}

	// main watcher event processing loop
	go func() {

		// set up worker pool semaphore, to prevent hitting file-handle limits
		type token struct{}
		var poolSize = 10 // max files to process concurrently
		sem := make(chan token, poolSize)

		for {
			select {
			case event := <-rdr.watcher.Event:
				if event.Op == watcher.Remove {
					fmt.Printf("\n\tfile:%s operation:%s %s\n", event.Name(), event.Op, time.Now())
				} else if (event.Op == watcher.Write || event.Op == watcher.Create) && event.IsDir() == false {
					fmt.Printf("\n\tfile:%s", event.Path)
					fmt.Printf("\n\toperation:%s modified: %s\n", event.Op, event.ModTime())
					sem <- token{}             // acquire pool slot
					go func(fileName string) { // spawn publishing worker
						err := rdr.publishFile(fileName)
						if err != nil {
							log.Println("error publishing file: ", fileName, err)
						}
						<-sem // release slot back to pool
						// timer block
						// benthos/jq in to detach allocations
					}(event.Path)
				}
			case err := <-rdr.watcher.Error:
				fmt.Println("\tFile-watcher error occurred: ", err)
				fmt.Println("File-watching suspended, recommend reader restart.")
				return
			case <-rdr.watcher.Closed:
				return
			}
		}
		// wait for all workers to complete
		// blocks until buffered channel can be filled to limit -
		// only possible once all workers have released back to pool
		for n := poolSize; n > 0; n-- {
			sem <- token{}
		}

	}()

	// Start the watching process.
	if err := rdr.watcher.Start(rdr.interval); err != nil {
		return err
	}

	return nil
}

func (rdr *OTFReader) publishFile(fileName string) error {
	fmt.Println("\n\tPUBLISHING:", fileName)

	var inputFile io.Reader
	inputFile, err := os.Open(fileName)
	if err != nil {
		return err
	}

	if rdr.inputFormat == "csv" {
		// if csv then first convert to json
		json, _ := n3csv.Reader2JSON(inputFile, "")
		fmt.Printf("\nconverted json:\n\n%s\n\n", json)
		// converter returns file as json string so re-wrap in reader
		inputFile = strings.NewReader(json)
	}

	// now read json file as stream, publish each object to nats
	d := json.NewDecoder(inputFile)

	// read opening brace "["
	_, err = d.Token()
	if err != nil {
		return errors.Wrap(err, "unexpected token; json file should be json array")
	}

	// for speed we're using async publishing in nats, which needs
	// a callback handler for any publishing errors, which is set up here
	ackHandler := func(ackedNuid string, err error) {
		if err != nil {
			log.Printf("Warning: error publishing msg id %s: %v\n", ackedNuid, err.Error())
		} else {
			// log.Printf("Received ack for msg id %s\n", ackedNuid)
		}
	}

	// read json objects one by one
	for d.More() {
		var m json.RawMessage
		err := d.Decode(&m)
		if err != nil {
			return errors.Wrap(err, "unable to decode json object.")
		}
		fmt.Printf("\n%s\n", m)
		// publish to nats
		nuid, err := rdr.sc.PublishAsync(rdr.publishTopic, m, ackHandler)
		if err != nil {
			log.Printf("Error publishing msg %s: %v\n", nuid, err.Error())
			return err
		}
	}

	return nil
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
