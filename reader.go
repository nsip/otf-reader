package otfreader

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
}

//
func New(options ...Option) (*OTFReader, error) {

	rdr := OTFReader{}

	if err := rdr.setOptions(options...); err != nil {
		return nil, err
	}

	return &rdr, nil
}
