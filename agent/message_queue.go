// BSD 2-Clause License
//
// Copyright (c) 2020, Andrea Giacomo Baldan
// All rights reserved.
//
// Redistribution and use in source and binary forms, with or without
// modification, are permitted provided that the following conditions are met:
//
// * Redistributions of source code must retain the above copyright notice, this
//   list of conditions and the following disclaimer.
//
// * Redistributions in binary form must reproduce the above copyright notice,
//   this list of conditions and the following disclaimer in the documentation
//   and/or other materials provided with the distribution.
//
// THIS SOFTWARE IS PROVIDED BY THE COPYRIGHT HOLDERS AND CONTRIBUTORS "AS IS"
// AND ANY EXPRESS OR IMPLIED WARRANTIES, INCLUDING, BUT NOT LIMITED TO, THE
// IMPLIED WARRANTIES OF MERCHANTABILITY AND FITNESS FOR A PARTICULAR PURPOSE ARE
// DISCLAIMED. IN NO EVENT SHALL THE COPYRIGHT HOLDER OR CONTRIBUTORS BE LIABLE
// FOR ANY DIRECT, INDIRECT, INCIDENTAL, SPECIAL, EXEMPLARY, OR CONSEQUENTIAL
// DAMAGES (INCLUDING, BUT NOT LIMITED TO, PROCUREMENT OF SUBSTITUTE GOODS OR
// SERVICES; LOSS OF USE, DATA, OR PROFITS; OR BUSINESS INTERRUPTION) HOWEVER
// CAUSED AND ON ANY THEORY OF LIABILITY, WHETHER IN CONTRACT, STRICT LIABILITY,
// OR TORT (INCLUDING NEGLIGENCE OR OTHERWISE) ARISING IN ANY WAY OUT OF THE USE
// OF THIS SOFTWARE, EVEN IF ADVISED OF THE POSSIBILITY OF SUCH DAMAGE.

package agent

import (
	"github.com/streadway/amqp"
)

type ProducerConsumer interface {
	Produce([]byte) error
	Consume(chan []byte) error
}

type AmqpQueue struct {
	url, queue                               string
	durable, deleteUnused, exclusive, noWait bool
}

type QueueOption func(*AmqpQueue)

func NewAmqpQueue(url, queueName string, opts ...QueueOption) *AmqpQueue {
	q := &AmqpQueue{
		url, queueName, false, false, false, false,
	}

	for _, opt := range opts {
		opt(q)
	}
	return q
}

// "amqp://guest:guest@localhost:5672/"

func (q AmqpQueue) Produce(item []byte) error {
	conn, err := amqp.Dial(q.url)
	if err != nil {
		return err
	}
	defer conn.Close()

	ch, err := conn.Channel()
	if err != nil {
		return err
	}
	defer ch.Close()

	queue, err := ch.QueueDeclare(
		q.queue,        // name
		q.durable,      // durable
		q.deleteUnused, // delete when unused
		q.exclusive,    // exclusive
		q.noWait,       // no-wait
		nil,            // arguments
	)
	if err != nil {
		return err
	}

	err = ch.Publish(
		"",         // exchange
		queue.Name, // routing key
		false,      // mandatory
		false,      // immediate
		amqp.Publishing{
			ContentType: "text/plain",
			Body:        item,
		},
	)
	if err != nil {
		return err
	}
	return nil
}

func (q AmqpQueue) Consume(itemChan chan []byte) error {
	conn, err := amqp.Dial(q.queue)
	if err != nil {
		return err
	}
	defer conn.Close()

	ch, err := conn.Channel()
	if err != nil {
		return err
	}
	defer ch.Close()

	queue, err := ch.QueueDeclare(
		q.queue,        // name
		q.durable,      // durable
		q.deleteUnused, // delete when unused
		q.exclusive,    // exclusive
		q.noWait,       // no-wait
		nil,            // arguments
	)
	if err != nil {
		return err
	}

	msgs, err := ch.Consume(
		queue.Name, // queue
		"",         // consumer
		true,       // auto-ack
		false,      // exclusive
		false,      // no-local
		false,      // no-wait
		nil,        // args
	)
	if err != nil {
		return err
	}

	forever := make(chan bool)

	go func() {
		for d := range msgs {
			itemChan <- d.Body
		}
	}()

	<-forever
	return nil
}
