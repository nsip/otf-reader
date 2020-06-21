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
	"github.com/tidwall/sjson"
)

type OtfReader struct {
	name            string
	ID              string
	providerName    string
	inputFormat     string
	levelMethod     string
	alignMethod     string
	genCapability   string
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
	concurrentFiles int
}

//
// create a new reader
//
func New(options ...Option) (*OtfReader, error) {

	rdr := OtfReader{}

	if err := rdr.setOptions(options...); err != nil {
		return nil, err
	}

	return &rdr, nil
}

//
// ensure graceful shutdown of file-watcher
//
func (rdr *OtfReader) Close() {
	rdr.sc.Close()
	rdr.watcher.Close()
}

//
// starts the reader monitoring the filesystem to
// harvest data files.
//
func (rdr *OtfReader) StartWatcher() error {

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
		sem := make(chan token, rdr.concurrentFiles)

		for {
			select {
			case event := <-rdr.watcher.Event:
				if event.Op == watcher.Remove {
					fmt.Printf("\nfile: %s\noperation: %s\nmodified: %s\n", event.Path, event.Op, time.Now())
				} else if (event.Op == watcher.Write || event.Op == watcher.Create) && event.IsDir() == false {
					fmt.Printf("\nfile: %s\noperation: %s\nmodified: %s\n", event.Path, event.Op, event.ModTime())
					sem <- token{}             // acquire pool slot
					go func(fileName string) { // spawn publishing worker
						err := rdr.publishFile(fileName)
						if err != nil {
							log.Println("error publishing file: ", fileName, err)
						}
						<-sem // release slot back to pool
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
		for n := rdr.concurrentFiles; n > 0; n-- {
			sem <- token{}
		}

	}()

	// Start the watching process.
	if err := rdr.watcher.Start(rdr.interval); err != nil {
		return err
	}

	return nil
}

//
// does the work of reading the input file, converting input to json
// then streaming otf format json records to nats.
// otf records contain original data and meta-data blocks.
//
func (rdr *OtfReader) publishFile(fileName string) error {

	fmt.Println("PUBLISHING:", fileName)
	defer util.TimeTrack(time.Now(), "publishFile()")

	var inputFile io.Reader
	inputFile, err := os.Open(fileName)
	if err != nil {
		return err
	}

	if rdr.inputFormat == "csv" {
		// if csv then first convert to json
		json, _ := n3csv.Reader2JSON(inputFile, "")
		// fmt.Printf("\nconverted json:\n\n%s\n\n", json)
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
	objCount := 0
	for d.More() {
		var m json.RawMessage
		err := d.Decode(&m)
		if err != nil {
			return errors.Wrap(err, "unable to decode json object.")
		}
		// insert the read data into the standard otf message
		otfMsg, err := sjson.SetRawBytes([]byte(""), "original", m)
		if err != nil {
			return errors.Wrap(err, "cannot add original json to otf message")
		}
		// now add the other meta-data
		otfMsg, err = sjson.SetRawBytes(otfMsg, "meta", rdr.metaBytes())
		if err != nil {
			return errors.Wrap(err, "cannot create meta-data block for otf message")
		}

		// fmt.Printf("\n-------------\n%s\n-----------\n", otfMsg)

		// publish to nats
		nuid, err := rdr.sc.PublishAsync(rdr.publishTopic, otfMsg, ackHandler)
		if err != nil {
			log.Printf("Error publishing msg %s: %v\n", nuid, err.Error())
			return err
		}
		objCount++
	}

	fmt.Printf("%d records published from %s\n", objCount, fileName)
	return nil
}

//
// constructs a json block containing values taken
// from the reader
//
func (rdr *OtfReader) metaBytes() []byte {

	metaString := fmt.Sprintf(`{
	"providerName": "%s",
	"inputFormat": "%s",
	"alignMethod": "%s",
	"levelMethod": "%s",
	"readerName": "%s",
	"readerID": "%s",
	"capability": "%s"
}`, rdr.providerName, rdr.inputFormat, rdr.alignMethod,
		rdr.levelMethod, rdr.name, rdr.ID, rdr.genCapability)

	return []byte(metaString)

}

//
// print the current config
//
func (rdr *OtfReader) PrintConfig() {

	fmt.Println("\n\tOTF-Reader Configuration")
	fmt.Println("\t------------------------\n")

	rdr.printID()
	rdr.printDataConfig()
	rdr.printNatsConfig()
	rdr.printWatcherConfig()

}

func (rdr *OtfReader) printID() {
	fmt.Println("\treader name:\t\t", rdr.name)
	fmt.Println("\treader ID:\t\t", rdr.ID)
}

func (rdr *OtfReader) printDataConfig() {
	fmt.Println("\tdata provider:\t\t", rdr.providerName)
	fmt.Println("\tinput format:\t\t", rdr.inputFormat)
	fmt.Println("\talign method:\t\t", rdr.alignMethod)
	fmt.Println("\tlevel method:\t\t", rdr.levelMethod)
	fmt.Println("\tgen-capability:\t\t", rdr.genCapability)
}

func (rdr *OtfReader) printNatsConfig() {
	fmt.Println("\tnats port:\t\t", rdr.natsPort)
	fmt.Println("\tnats host:\t\t", rdr.natsHost)
	fmt.Println("\tnats cluster-id:\t", rdr.natsCluster)
	fmt.Println("\tnats topic:\t\t", rdr.publishTopic)
}

func (rdr *OtfReader) printWatcherConfig() {
	fmt.Println("\twatch file suffix:\t", rdr.watchFileSuffix)
	fmt.Println("\twatch poll interval:\t", rdr.interval)
	fmt.Println("\twatch dot files:\t", rdr.dotfiles)
	fmt.Println("\tignore files:\t\t", rdr.ignore)
	fmt.Println("\twatch folder:\t\t", rdr.watchFolder)
	fmt.Println("\tmax concurrent files:\t\t", rdr.concurrentFiles)
	fmt.Println("\tfiles being watched:")
	for path, f := range rdr.watcher.WatchedFiles() {
		// fmt.Printf("\t   %s: %s\n", path, f.Name())
		_ = path
		fmt.Printf("\t\t\t%s\n", f.Name())
	}
	fmt.Println()

}
