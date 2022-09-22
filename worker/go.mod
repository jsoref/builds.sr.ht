module git.sr.ht/~sircmpwn/builds.sr.ht/worker

require (
	git.sr.ht/~sircmpwn/core-go v0.0.0-20210108160653-070566136c1a
	github.com/go-redis/redis/v8 v8.2.3
	github.com/gocelery/gocelery v0.0.0-20201111034804-825d89059344
	github.com/google/shlex v0.0.0-20191202100458-e7afc7fbc510
	github.com/kr/pty v1.1.3
	github.com/lib/pq v1.8.0
	github.com/martinlindhe/base36 v1.1.0
	github.com/minio/minio-go/v6 v6.0.49
	github.com/mitchellh/mapstructure v1.1.2
	github.com/pkg/errors v0.9.1
	github.com/prometheus/client_golang v1.13.0
	github.com/streadway/amqp v1.0.0 // indirect
	github.com/vaughan0/go-ini v0.0.0-20130923145212-a98ad7ee00ec
	gopkg.in/alexcesaro/quotedprintable.v3 v3.0.0-20150716171945-2caba252f4dc // indirect
	gopkg.in/mail.v2 v2.3.1
	gopkg.in/yaml.v2 v2.4.0
)

go 1.13
