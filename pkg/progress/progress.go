package progress

import (
	"fmt"
	"os"
	"strings"
	"time"
)

type Progress struct {
	TotalSteps int
	CurrentStep int
	Message string
	StartTime time.Time
}

func NewProgress(totalSteps int) *Progress {
	return &Progress{
		TotalSteps: totalSteps,
		CurrentStep: 0,
		StartTime: time.Now(),
	}
}

func (p *Progress) Update(message string) {
	p.CurrentStep++
	p.Message = message
	p.print()
}

func (p *Progress) print() {
	// Clear the current line
	fmt.Print("\r\033[K")
	
	// Calculate progress percentage
	percentage := float64(p.CurrentStep) / float64(p.TotalSteps) * 100
	
	// Create progress bar
	barWidth := 50
	completed := int(float64(barWidth) * percentage / 100)
	if completed < 0 {
		completed = 0
	}
	if completed > barWidth {
		completed = barWidth
	}
	bar := strings.Repeat("=", completed) + strings.Repeat(" ", barWidth-completed)
	
	// Calculate elapsed time
	elapsed := time.Since(p.StartTime).Round(time.Second)
	
	// Print progress
	fmt.Printf("[%s] %.1f%% %s (%s elapsed)", bar, percentage, p.Message, elapsed)
	
	// If this is the last step, print a newline
	if p.CurrentStep == p.TotalSteps {
		fmt.Println()
	}
}

func (p *Progress) Success() {
	p.print()
	fmt.Println("✅ Operation completed successfully!")
}

func (p *Progress) Error(err error) {
	p.print()
	fmt.Printf("❌ Operation failed: %v\n", err)
	os.Exit(1)
} 