package jobs

import (
	"fmt"
	"log"
	"time"

	"server/logs"

	"github.com/robfig/cron/v3"
)

var scheduler *cron.Cron

type Job struct {
	Spec string
	Func func()
	Name string
}

func Init() error {
	loc, err := time.LoadLocation("America/Detroit")
	if err != nil {
		return fmt.Errorf("failed to load timezone: %v", err)
	}

	scheduler = cron.New(
		cron.WithLocation(loc),
		cron.WithSeconds(),
		cron.WithChain(cron.Recover(cron.DefaultLogger)),
	)

	addJobs()

	scheduler.Start()
	return nil
}

func registerJobs(jobs []Job) {
	for _, job := range jobs {
		_, err := scheduler.AddFunc(job.Spec, func() {
			if job.Name != "" {
				logs.INFO("Running scheduled "+job.Name, nil)
			}
			job.Func()
		})
		if err != nil {
			log.Fatalf("failed to schedule %s: %v", job.Name, err)
		}
	}
}

func addJobs() {
	jobs := []Job{
		{
			Spec: "0 7 9 * * *",
			Func: GetWordle,
			Name: "GetWordle",
		},
		{
			Spec: "0 7 9 * * *",
			Func: ScrapePetrarchan,
			Name: "ScrapePetrarchan AM",
		},
		{
			Spec: "0 7 21 * * *",
			Func: ScrapePetrarchan,
			Name: "ScrapePetrarchan PM",
		},
	}

	registerJobs(jobs)
}
