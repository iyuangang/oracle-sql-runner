package utils

import (
	"fmt"
	"sync"
	"time"

	"github.com/schollz/progressbar/v3"
)

type Progress struct {
	bar     *progressbar.ProgressBar
	total   int
	current int
	mu      sync.Mutex
	start   time.Time
}

func NewProgress(total int, description string) *Progress {
	return &Progress{
		bar: progressbar.NewOptions(total,
			progressbar.OptionEnableColorCodes(true),
			progressbar.OptionShowBytes(false),
			progressbar.OptionSetWidth(15),
			progressbar.OptionSetDescription(description),
			progressbar.OptionSetTheme(progressbar.Theme{
				Saucer:        "[green]=[reset]",
				SaucerHead:    "[green]>[reset]",
				SaucerPadding: " ",
				BarStart:      "[",
				BarEnd:        "]",
			}),
		),
		total: total,
		start: time.Now(),
	}
}

func (p *Progress) Increment() {
	p.mu.Lock()
	defer p.mu.Lock()
	p.current++
	p.bar.Add(1)
}

func (p *Progress) SetCurrent(current int) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.current = current
	p.bar.Set(current)
}

func (p *Progress) Finish() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.bar.Finish()
	duration := time.Since(p.start)
	fmt.Printf("\n完成执行 %d 个语句，耗时：%s\n",
		p.current, FormatDuration(duration))
}
