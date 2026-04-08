package event

type Handler func(any)

type Dispatcher struct {
	triggerHandlers map[string][]Handler
	queueHandlers   map[string][]Handler
	queue           []queuedEvent
}

type queuedEvent struct {
	name    string
	payload any
}

func NewDispatcher() *Dispatcher {
	return &Dispatcher{
		triggerHandlers: make(map[string][]Handler),
		queueHandlers:   make(map[string][]Handler),
	}
}

func (d *Dispatcher) SubscribeTrigger(name string, handler Handler) {
	d.triggerHandlers[name] = append(d.triggerHandlers[name], handler)
}

func (d *Dispatcher) SubscribeQueue(name string, handler Handler) {
	d.queueHandlers[name] = append(d.queueHandlers[name], handler)
}

func (d *Dispatcher) Trigger(name string, payload any) {
	for _, handler := range d.triggerHandlers[name] {
		handler(payload)
	}
}

func (d *Dispatcher) Enqueue(name string, payload any) {
	d.queue = append(d.queue, queuedEvent{name: name, payload: payload})
}

func (d *Dispatcher) Update() {
	events := d.queue
	d.queue = nil
	for _, evt := range events {
		for _, handler := range d.queueHandlers[evt.name] {
			handler(evt.payload)
		}
	}
}
