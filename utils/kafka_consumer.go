package utils

import (
	"os"
	"os/signal"

	"github.com/Shopify/sarama"
	cluster "github.com/bsm/sarama-cluster"
	"github.com/sirupsen/logrus"
)

// Msg : parse kafka msg
type Msg interface {
	ParseKafkaMsg(message *sarama.ConsumerMessage) error
}

/*
Consumer : return consumer msg chan
args:
	brokers						| kafka addresses
	consumerGroup   			| kafka consumer group
	topic						| kafka topic
	newMsg(topic string) Msg 	| get diffrent Msg object by diffrent kafka topic
	logger						| log hardle
	success						| msg parse success num to print or log
	fail						| msg parse fail num to print or log
*/
func Consumer(brokers []string, consumerGroup string, topics []string, newMsg func(topic string) Msg, logger *logrus.Logger, success, fail int) {
	logger.Debug("Assoc Consumer:", brokers, consumerGroup, topics, success, fail)

	// cluster config
	config := cluster.NewConfig()
	config.Consumer.Return.Errors = true
	config.Group.Return.Notifications = true
	config.Consumer.Offsets.Initial = sarama.OffsetNewest

	// create consumer
	consumer, err := cluster.NewConsumer(brokers, consumerGroup, topics, config)
	if err != nil {
		logger.Errorf("%s create consumer error,%s", consumerGroup, err.Error())
	}
	defer consumer.Close()

	// rebalanced msg
	go func() {
		for ntf := range consumer.Notifications() {
			logger.Debugf("%s: reblanced: %v \n", consumerGroup, ntf)
		}
	}()

	// signal interrupt
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt)

	var successCount, failCount int
	topic := topics[0]

loop:
	for {
		select {
		// reply msg
		case msg, ok := <-consumer.Messages():
			if ok {
				consumer.MarkOffset(msg, "")
				kafkaMsg := newMsg(topic)
				if err := kafkaMsg.ParseKafkaMsg(msg); err != nil && err.Error() != "" {
					failCount++
					if failCount%fail == 0 {
						logger.Errorf("%s receive error msg： %s Error:%v", topic, string(msg.Value), err)
					}
				} else {
					successCount++
					if successCount%success == 0 {
						logger.Debugf("%s receive msg: %s", topic, string(msg.Value))
					}
				}
			}
		case <-signals:
			logger.Infof("%s restart kafka [%s]", consumerGroup, topic)
			goto loop
		}
	}
}

/*
MultAssocComsumer : run more assco to consumer one topic
args:
	assocCount					| assoc num
	brokers						| kafka addresses
	consumerGroup   			| kafka consumer group
	topic						| kafka topic
	newMsg(topic string) Msg 	| get diffrent Msg object by diffrent kafka topic
	logger						| log hardle
	success						| msg parse success num to print or log
	fail						| msg parse fail num to print or log
*/
func MultAssocComsumer(assocCount int, brokers []string, consumerGroup string, topic string, newMsg func(topic string) Msg, logger *logrus.Logger, success, fail int) {
	for i := 0; i < assocCount; i++ {
		logger.Debug("start kafka.Consumer ", i)
		go func() {
			Consumer(brokers, consumerGroup, []string{topic}, newMsg, logger, success, fail)
		}()
	}
}