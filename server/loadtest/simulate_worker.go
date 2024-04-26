package loadtest

type PostToChatJob struct {
	channelId string
	msUserId  string
	message   string
	count     int
	total     int
}

var SimulateQueue chan PostToChatJob

type SimulateWorker struct {
	WorkerPool      chan chan PostToChatJob
	SimulateChannel chan PostToChatJob
	quit            chan bool
}

func init() {
	if Settings.Enabled {
		SimulateQueue = make(chan PostToChatJob, 1000)
	}
}

func NewSimulateWorker(workerPool chan chan PostToChatJob) SimulateWorker {
	return SimulateWorker{
		WorkerPool:      workerPool,
		SimulateChannel: make(chan PostToChatJob),
		quit:            make(chan bool),
	}
}

func (w SimulateWorker) Start() {
	go func() {
		for {
			w.WorkerPool <- w.SimulateChannel

			select {
			case job := <-w.SimulateChannel:
				log("Worker Job")
				simulatePostToChat(job)

			case <-w.quit:
				return
			}
		}
	}()
}

func (w SimulateWorker) Stop() {
	go func() {
		w.quit <- true
	}()
}

type Dispatcher struct {
	WorkerPool chan chan PostToChatJob
	maxWorkers int
}

func NewDispatcher(maxWorkers int) *Dispatcher {
	pool := make(chan chan PostToChatJob, maxWorkers)
	return &Dispatcher{
		WorkerPool: pool,
		maxWorkers: maxWorkers,
	}
}

func (d *Dispatcher) Run() {
	for i := 0; i < d.maxWorkers; i++ {
		worker := NewSimulateWorker(d.WorkerPool)
		worker.Start()
	}

	go d.dispatch()
}

func (d *Dispatcher) dispatch() {
	for job := range SimulateQueue {
		go func(job PostToChatJob) {
			jobChannel := <-d.WorkerPool
			jobChannel <- job
		}(job)
	}
}
