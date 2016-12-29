package main

import (
	"bufio"
	"crypto/tls"
	"crypto/x509"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"

	"github.com/streadway/amqp"
)

const (
	RMQ_CLI_VERSION = "1.0.0"
)

type ConfigOpts struct {
	printVersion bool

	host     string
	username string
	password string
	queue    string
	exchange string
	vhost    string
	tlsCert  string
	tlsKey   string
	tlsCa    string
	useTls   bool

	publish bool

	consume        bool
	consumeRequeue bool
	consumeCount   int

	delimiter string
}

/**
 * validate the the config we got from flags is valid
 */
func (opts *ConfigOpts) validate() {
	if (opts.publish && opts.consume) || (!opts.publish && !opts.consume) {
		log.Fatalf("Must publish or consume but not both, aborting")
	}

	if opts.publish && (opts.queue == "" && opts.exchange == "") {
		log.Fatalf("Must specify a queue or exchange to publish to")
	}

	if opts.publish && (opts.queue != "" && opts.exchange != "") {
		log.Fatalf("Must specify a queue or exchange to publish to, not both")
	}

	if len(opts.delimiter) != 1 {
		log.Fatalf("Delimiter must be a single character")
	}

	if opts.consume && opts.queue == "" {
		log.Fatalf("Must specify a queue to consume from")
	}

	if opts.username == "" {
		log.Fatalf("Must specify a username to connect with")
	}

	if opts.password == "" {
		log.Fatalf("Must specify a password to connect with")
	}

	if opts.useTls && (opts.tlsCert == "" || opts.tlsKey == "") {
		log.Fatalf("Must supply tlsCert and tlsKey if using tls")
	}
}

func fatalIfErr(err error, msg string) {
	if err != nil {
		log.Fatalf("%s: %v", msg, err)
	}
}

func initFlags(opts *ConfigOpts) {

	// helpers
	flag.BoolVar(&opts.printVersion, "version", false, "Prints rmq-cli version")

	// connection opts
	flag.StringVar(&opts.host, "host", "localhost:5672", "Host to connect to rabbitmq instance")
	flag.StringVar(&opts.username, "username", "",
		"Username to connect with (if not supplied, will attempt to load from env $RMQ_USERNAME)")
	flag.StringVar(&opts.password, "password", "",
		"Password to connect with (if not supplied, will attempt to load from env $RMQ_PASSWORD)")
	flag.StringVar(&opts.tlsCert, "tlsCert", "",
		"Path to TLS certificate (if not supplied, will attempt to load from env $RMQ_CERT_PATH)")
	flag.StringVar(&opts.tlsKey, "tlsKey", "",
		"Path to TLS key (if not supplied, will attempt to load from env $RMQ_KEY_PATH)")
	flag.StringVar(&opts.tlsCa, "tlsCa", "",
		"Path to TLS CA (if not supplied, will attempt to load from env $RMQ_CA_PATH)")
	flag.BoolVar(&opts.useTls, "useTls", false, "Connect using TLS")

	// rmq opts
	flag.StringVar(&opts.queue, "queue", "", "Queue to publish/consume to/from")
	flag.StringVar(&opts.exchange, "exchange", "", "Queue to publish to")
	flag.StringVar(&opts.vhost, "vhost", "/", "VHost to connect to")

	// publish opts
	flag.BoolVar(&opts.publish, "publish", false, "Set to publish mode")

	// consume opts
	flag.BoolVar(&opts.consume, "consume", false, "Set to consume mode")
	flag.BoolVar(&opts.consumeRequeue, "requeue", true, "Requeue after consuming?")
	flag.IntVar(&opts.consumeCount, "n", 0,
		"Max number of messages to consume before disconnecting (set to -1 to consume all messages, defaults to 0)")

	// common opts
	flag.StringVar(&opts.delimiter, "d", "\n", "Delimiter to break Stdin/Stdout into messages")
}

func createRmqConnection(opts *ConfigOpts) (*amqp.Connection, error) {
	connectionString := fmt.Sprintf("amqp://%v:%v@%v/%v", opts.username, opts.password, opts.host, opts.vhost)
	log.Printf("Connecting to rmq at %v\n", connectionString)

	if opts.useTls {
		certBytes, err := ioutil.ReadFile(opts.tlsCert)
		fatalIfErr(err, "Failed to load tls certificate bytes")
		keyBytes, err := ioutil.ReadFile(opts.tlsKey)
		fatalIfErr(err, "Failed to load tls key bytes")

		cfg := new(tls.Config)
		cfg.MinVersion = tls.VersionTLS12
		cfg.MaxVersion = tls.VersionTLS12
		cfg.PreferServerCipherSuites = true
		cfg.CipherSuites = []uint16{
			tls.TLS_RSA_WITH_AES_256_CBC_SHA,
			tls.TLS_RSA_WITH_AES_128_CBC_SHA,
			tls.TLS_ECDHE_ECDSA_WITH_AES_128_CBC_SHA,
			tls.TLS_ECDHE_ECDSA_WITH_AES_256_CBC_SHA,
			tls.TLS_ECDHE_RSA_WITH_AES_128_CBC_SHA,
			tls.TLS_ECDHE_RSA_WITH_AES_256_CBC_SHA,
			tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
			tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256}

		cfg.RootCAs = x509.NewCertPool()

		if opts.tlsCa != "" {
			caBytes, err := ioutil.ReadFile(opts.tlsCa)
			fatalIfErr(err, "Failed to load tls ca bytes")
			cfg.RootCAs.AppendCertsFromPEM(caBytes)
		}

		pair, err := tls.X509KeyPair(certBytes, keyBytes)
		fatalIfErr(err, "Unable to create keypair from cert/key")
		cfg.Certificates = []tls.Certificate{pair}

		return amqp.DialTLS(connectionString, cfg)
	} else {
		return amqp.Dial(connectionString)
	}
}

func consume(opts *ConfigOpts, conn *amqp.Connection) {
	channel, err := conn.Channel()
	fatalIfErr(err, "Unable open channel on amqp connection")

	// if we're requeueing we have to set QOS to at least the consume count + 1so we can hold all of the messages before we kill our connection allowing them to be requeued
	if opts.consumeCount > 0 && opts.consumeRequeue {
		channel.Qos(opts.consumeCount+1, 0, false)
	} else {
		channel.Qos(1000, 0, false)
	}

	tag := "rmq-cli-consumer"
	defer channel.Cancel(tag, true)

	deliveries, err := channel.Consume(
		opts.queue, // name
		tag,        // consumerTag,
		false,      // autoAck
		false,      // exclusive
		false,      // noLocal
		false,      // noWait
		nil,        // arguments
	)

	fatalIfErr(err, "Unable consume from channel")

	consumeCloseChan := make(chan bool)

	count := 0
	go func() {
		for d := range deliveries {

			os.Stdout.Write(d.Body)
			fmt.Fprintf(os.Stdout, opts.delimiter)

			if opts.consumeRequeue == false {
				d.Ack(false)
			}

			count += 1

			if opts.consumeCount > -1 && count >= opts.consumeCount {
				consumeCloseChan <- true
				break
			}
		}

		consumeCloseChan <- true
	}()

	<-consumeCloseChan
	log.Printf("Finished consuming %v messages\n", count)
}

func publish(opts *ConfigOpts, conn *amqp.Connection) {
	channel, err := conn.Channel()
	fatalIfErr(err, "Unable open channel on amqp connection")

	tag := "rmq-cli-publisher"
	defer channel.Cancel(tag, true)

	inputReader := bufio.NewReader(os.Stdin)
	count := 0
	delimiterByte := opts.delimiter[0]
	for {
		inputBytes, err := inputReader.ReadBytes(delimiterByte)
		eof := (err == io.EOF)

		if err == nil || (eof && len(inputBytes) > 1) {
			err = channel.Publish(
				opts.exchange,
				opts.queue,
				false,
				false,
				amqp.Publishing{
					Headers:         amqp.Table{},
					ContentType:     "text/plain",
					ContentEncoding: "",
					Body:            inputBytes[:len(inputBytes)-1],
					DeliveryMode:    amqp.Persistent,
					Priority:        0,
				},
			)
			count += 1
		}

		// check if we reached end of input, if so short circuit
		if eof {
			log.Printf("Reached end of input, exiting cleanly, published %v messages", count)
			break
		}

		// if we have any other error we need to exit
		fatalIfErr(err, fmt.Sprintf("Had problem reading input after %v messages, exiting", count))
	}

}

func main() {
	var opts ConfigOpts
	initFlags(&opts)
	flag.Parse()

	if opts.printVersion {
		log.Printf("rmq-cli - Version: %v", RMQ_CLI_VERSION)
		os.Exit(2)
	}

	if opts.username == "" {
		opts.username = os.Getenv("RMQ_USERNAME")
		if opts.username == "" {
			opts.username = "guest"
		}
	}

	if opts.password == "" {
		opts.password = os.Getenv("RMQ_PASSWORD")
		if opts.password == "" {
			opts.password = "guest"
		}
	}

	if opts.useTls {
		opts.tlsCert = os.Getenv("RMQ_CERT_PATH")
		opts.tlsKey = os.Getenv("RMQ_KEY_PATH")
		opts.tlsCa = os.Getenv("RMQ_CA_PATH")
	}

	// validate opts
	opts.validate()

	log.Printf("Starting rmq cli with opts %+v\n", opts)

	conn, err := createRmqConnection(&opts)
	fatalIfErr(err, "Unable to connect to rmq")

	defer conn.Close()

	if opts.publish {
		publish(&opts, conn)
	} else if opts.consume {
		consume(&opts, conn)
	}
}
