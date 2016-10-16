# rmq-cli publisher/consumer
What rmq-cli does is provide a streamlined way to consume to STDOUT and publish from STDIN.
Features include:
- TLS support
- consume n messages from a given queue
- requeue messages consumed
- publish to exchange or queue
- custom delimiters for separating messages in input/output

## Why?
A common problem when running rabbitmq is you need to (for random reasons) consume a certain amount of messages from a queue and publish them somewhere else. Sometimes this is due to a certain number of messages having been poisoned and need to be replayed. Or other times you have a test workload of messages that need to be published to simulate work. Unfortunately there aren't really good adhoc tools for this. `rabbitmqadmin` does exist, but unfortunately only can publish a single message at a time and the consume process leaves a lot to be desired as well. rmq-cli attempts to bridge this gap by providing a simple cli tool for consuming and publishing messages.

## Usage
```
$ rmq-cli -h

-consume
    Set to consume mode
-d string
    Delimiter to break Stdin/Stdout into messages (default "\n")
-exchange string
    Queue to publish to
-host string
    Host to connect to rabbitmq instance (default "localhost:5672")
-n int
    Max number of messages to consume before disconnecting (set to -1 to consume all messages, defaults to 0)
-password string
    Password to connect with (if not supplied, will attempt to load from env $RMQ_PASSWORD)
-publish
    Set to publish mode
-queue string
    Queue to publish/consume to/from
-requeue
    Requeue after consuming? (default true)
-tlsCa string
    Path to TLS CA (if not supplied, will attempt to load from env $RMQ_CA_PATH)
-tlsCert string
    Path to TLS certificate (if not supplied, will attempt to load from env $RMQ_CERT_PATH)
-tlsKey string
    Path to TLS key (if not supplied, will attempt to load from env $RMQ_KEY_PATH)
-useTls
    Connect using TLS
-username string
    Username to connect with (if not supplied, will attempt to load from env $RMQ_USERNAME)
-version
    Prints rmq-cli version
-vhost string
    VHost to connect to (default "/")
```

#### Consume
Consume 50 messages and dump to file

`$ rmq-cli -consume -queue=test_queue -n 50 -requeue=false > messages.txt`

**Note** if you specify a number of messages to consume which is larger than the number of messages in the given queue, the consumer will consume all messages and continue to listen for more. rmq-cli will ack after every message consumed if `-requeue=false` so at this point you can kill the consumer with `^c`. However, it is better to specify the exact number of messages in the queue you wish to consume as that will result in a cleaner shutdown, especially if you are piping the results of the consumer into a publisher.

#### Publish
Publish messages from file to exchange

`$ cat messages.txt > rmq-cli -publish -exchange=test_exchange`

Publish messages from file to queue

`$ cat messages.txt > rmq-cli -publish -queue=test_queue`

#### Consume > Publish
`$ rmq-cli -consume -queue=test_poison_queue -n 50 -requeue=false | mq-cli -publish -queue=test_queue`

#### Host/username/password
You can set the rmq host, username, and password using the following cli opts or environment variables
- `-host` - amqp hostname + port
- `-username` or `RMQ_USERNAME` - username to login to rmq with
- `-password` or `RMQ_PASSWORD` - password to login to rmq with

#### TLS
If your RMQ server is setup to do TLS you can use the following cli opts or environment variables to setup key/cert/ca.
- `-tlsCert` or `RMQ_CERT_PATH` - path to ssl.crt
- `-tlsKey` or `RMQ_KEY_PATH` - path to ssl.key
- `-tlsCa` or `RMQ_CA_PATH` - path to ssl ca

#### Delimiter
By default, rmq-cli will separate messages going to STDOUT when consuming and separate messages coming from STDIN using a delimeter (default `\n`). This is configurable in case your data needs to be delimited by a different character (as in the case of messages that could contain line breaks).

## Administrative

### Prebuilt version
Go to GH, download latest rmq-cli binary from Releases

### Building
To build, first setup your `GOPATH`
Then either run `go get github.banksimple.com/joekarl/rmq-cli` or clone manually into your `GOPATH`.

This project uses godep to manage dependencies, if your dependencies are out of data, run `godep restore` prior to building/running.

### Packaging
Run `go build` to build a binary of this project. Run `go install` to install this in the bin of your `GOPATH`
