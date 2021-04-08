package otfreader

import (
	"os"
	"regexp"
	"strings"
	"time"

	util "github.com/nsip/otf-util"
	"github.com/pkg/errors"
	"github.com/radovskyb/watcher"
)

type Option func(*OtfReader) error

//
// apply all supplied options to the reader
// returns any error encountered while applying the options
//
func (rdr *OtfReader) setOptions(options ...Option) error {
	for _, opt := range options {
		if err := opt(rdr); err != nil {
			return err
		}
	}
	return nil
}

//
// set a name for this instance of an otf reader, if none
// provided a hashid will be created by default
//
func Name(name string) Option {
	return func(rdr *OtfReader) error {
		if name != "" {
			rdr.name = name
			return nil
		}
		rdr.name = util.GenerateName("otf-reader")
		return nil
	}
}

//
// create a unique id for the reader, if none
// provided a nuid will be generated by default
//
func ID(id string) Option {
	return func(rdr *OtfReader) error {
		if id != "" {
			rdr.ID = id
			return nil
		}
		rdr.ID = util.GenerateID()
		return nil
	}
}

//
// name of the external system that created the
// data being read
//
func ProviderName(name string) Option {
	return func(rdr *OtfReader) error {
		if name != "" {
			rdr.providerName = name
			return nil
		}
		rdr.providerName = "unspecified"
		return nil
	}
}

//
// the format of the input data, currently supported foramts
// are; csv, json & xml
//
func InputFormat(iformat string) Option {
	return func(rdr *OtfReader) error {
		if iformat == "" {
			return errors.New("otf-reader InputFormat cannot be empty.")
		}

		format := strings.ToLower(iformat)
		trimFormat := strings.Trim(format, ".") // remove any ecess . chars
		switch trimFormat {
		case "csv", "json":
			rdr.inputFormat = trimFormat
			return nil
		}
		return errors.New("otf-reader InputFormat " + iformat + " not supported (must be one of csv|json)")
	}
}

//
// select the levelling/scaling method appropriate for data from this vendor
// can be one of
// prescribed: input data specifies level
// mapped-scale: uses external scale such as NAPLAN
// rules: uses aggregation rules such as 3 observations of indicator required to indicate success
//
func LevelMethod(lmethod string) Option {
	return func(rdr *OtfReader) error {
		if lmethod == "" {
			return errors.New("otf-reader LevelMethod cannot be empty.")
		}

		method := strings.ToLower(lmethod)
		switch method {
		case "prescribed", "mapped", "rules":
			rdr.levelMethod = method
			return nil
		}
		return errors.New("otf-reader LevelMethod " + lmethod + " not supported (must be one of prescribed|mapped|rules)")
	}
}

func Capability(genCap string) Option {
	return func(rdr *OtfReader) error {
		if genCap == "" {
			return errors.New("otf-reader General Capability cannot be empty.")
		}

		capb := strings.ToLower(genCap)
		switch capb {
		case "literacy", "numeracy":
			rdr.genCapability = capb
			return nil
		}
		return errors.New("otf-reader General Capability " + genCap + " not supported (must be one of literacy|numeracy)")
	}
}

//
// select the alignment method appropriate data from this vendor
// can be one of
// prescribed: input data specifies alignment to NLP
// mapped: uses external alignment mapping such as through AC
// inferred: send data to text classifier to estbalish alignment
//
func AlignMethod(amethod string) Option {
	return func(rdr *OtfReader) error {
		if amethod == "" {
			return errors.New("otf-reader AlignMethod cannot be empty.")
		}

		method := strings.ToLower(amethod)
		switch method {
		case "prescribed", "mapped", "inferred":
			rdr.alignMethod = method
			return nil
		}
		return errors.New("otf-reader AlignMethod " + amethod + " not supported (must be one of prescribed|mapped|inferred)")
	}
}

//
// set the nats server communication port
// port value of 0 or less will result in default nats port 4222
//
func NatsPort(port int) Option {
	return func(rdr *OtfReader) error {
		if port > 0 {
			rdr.natsPort = port
			return nil
		}
		rdr.natsPort = 4222 //nats default
		return nil
	}
}

//
// set the nats server name or ip address
// empty string will result in localhost as defalt hostname
//
func NatsHostName(hostName string) Option {
	return func(rdr *OtfReader) error {
		if hostName != "" {
			rdr.natsHost = hostName
		}
		rdr.natsHost = "localhost" //nats default
		return nil
	}
}

//
// set the nats streaming server cluster name.
// empty string will result in nats default of 'test-cluster'
//
func NatsClusterName(clusterName string) Option {
	return func(rdr *OtfReader) error {
		if clusterName != "" {
			rdr.natsCluster = clusterName
		}
		rdr.natsCluster = "test-cluster" //nats default
		return nil
	}
}

//
// set the name of the nats topic to publish data once parsed
// from the input files
//
func TopicName(tName string) Option {
	return func(rdr *OtfReader) error {
		if tName == "" {
			return errors.New("must have TopicName (nats topic to which reader will publish parsed data).")
		}

		// topic regex check
		ok, err := util.ValidateNatsTopic(tName)
		if ok {
			rdr.publishTopic = tName
			return nil
		}
		return errors.Wrap(err, "TopicName option error")
	}

}

//
// set the number of input files that can be handled concurrently
// set if number of filehandles on OS is a problem
// defaults to 10
//
func ConcurrentFiles(n int) Option {
	return func(rdr *OtfReader) error {
		if n == 0 {
			rdr.concurrentFiles = 10 // safe default
			return nil
		}
		rdr.concurrentFiles = n
		return nil
	}

}

//
// configure the internal file watcher
//
func Watcher(folder string, fileSuffix string, interval string, recursive bool, dotfiles bool, ignore string) Option {
	return func(rdr *OtfReader) error {

		rdr.watcher = watcher.New()

		// dot file handling
		rdr.watcher.IgnoreHiddenFiles(!dotfiles)
		rdr.dotfiles = dotfiles

		// If no files/folders were specified, watch the current directory.
		if folder == "" {
			var osErr error
			folder, osErr = os.Getwd()
			if osErr != nil {
				return errors.Wrap(osErr, "no watch folder specified, and cannot determine current working diectory")
			}
		}
		rdr.watchFolder = folder

		// Get any of the paths to ignore.
		ignoredPaths := strings.Split(ignore, ",")
		for _, path := range ignoredPaths {
			trimmed := strings.TrimSpace(path)
			if trimmed == "" {
				continue
			}

			err := rdr.watcher.Ignore(trimmed)
			if err != nil {
				return errors.Wrap(err, "unable to add ignore folder "+trimmed)
			}
		}
		rdr.ignore = ignore

		// Only files that match the regular expression for file suffix during file listings
		// will be watched.
		if fileSuffix != "" {
			trimSuffix := strings.Trim(fileSuffix, ".")
			r := regexp.MustCompile("([^\\s]+(\\.(?i)(" + trimSuffix + "))$)")
			rdr.watcher.AddFilterHook(watcher.RegexFilterHook(r, false))
		}
		rdr.watchFileSuffix = fileSuffix

		// Add the watch folder specified.
		if recursive {
			if err := rdr.watcher.AddRecursive(folder); err != nil {
				return errors.Wrap(err, "unable to add watch folder "+folder+" recursively")
			}
		} else {
			if err := rdr.watcher.Add(folder); err != nil {
				return errors.Wrap(err, "unable to add watch folder "+folder)
			}
		}
		rdr.recursive = recursive

		// Parse the interval string into a time.Duration.
		parsedInterval, err := time.ParseDuration(interval)
		if err != nil {
			return errors.Wrap(err, "unable to parse watcher interval as duration")
		}
		rdr.interval = parsedInterval

		return nil
	}

}
