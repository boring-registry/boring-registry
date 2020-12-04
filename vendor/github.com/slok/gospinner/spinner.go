package gospinner

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"
)

var (
	// default colors for the application
	defaultColor        = FgHiCyan
	defaultSuccessColor = FgHiGreen
	defaultFailColor    = FgHiRed
	defaultWarnColor    = FgHiYellow
)

// Spinner is a representation of the animation itself
type Spinner struct {

	// Writer will be the target of the printing
	Writer io.Writer

	// frames are the frames that will be showed on screen, they are a representation of animation+runes
	frames []string

	// message is the content wanted to show with the loading animation
	message string

	// chars are the animation characters
	animation Animation

	// step tracks the current step
	step int

	// ticker is the animation ticker, will set the pace
	ticker *time.Ticker

	// Previous frame is used to clean the screen
	previousFrame string // TODO use  bytes so we dont allocate new strings always

	// running shows the state of the animation
	running bool

	// Our locker
	sync.Mutex

	// Separator will separate the messages each other, by default this should be carriage return
	separator string

	// colors
	color        *Color
	succeedColor *Color
	failColor    *Color
	warnColor    *Color

	// disableColor
	disableColor bool
}

//create is a helper function for all the creators
func create(kind AnimationKind) (*Spinner, error) {
	an, ok := animations[kind]
	if !ok {
		return nil, errors.New("Wrong kind of animation")
	}
	s := &Spinner{
		animation: an,
		Writer:    os.Stdout,
		separator: "\r",
		Mutex:     sync.Mutex{},
	}
	return s, nil
}

// NewSpinner creates a new spinner with the common default values, this should
// be the most used one, fast and easy.
func NewSpinner(kind AnimationKind) (*Spinner, error) {
	return NewSpinnerWithColor(kind, defaultColor)
}

// NewSpinnerNoColor creates an spinner that doesn't have color, shoud be
// compatible with all the terminals
func NewSpinnerNoColor(kind AnimationKind) (*Spinner, error) {
	s, err := NewSpinner(kind)
	// Disable colors
	s.disableColor = true
	s.color.DisableColor()
	s.succeedColor.DisableColor()
	s.failColor.DisableColor()
	s.warnColor.DisableColor()

	return s, err
}

// NewSpinnerWithColor creates an spinner with a custom color, same as the default
// one, but instead you can select the color you want for the spinner
func NewSpinnerWithColor(kind AnimationKind, color ColorAttr) (*Spinner, error) {
	s, err := create(kind)
	if err != nil {
		return nil, err
	}

	s.color = newColor(color)
	s.succeedColor = newColor(defaultSuccessColor)
	s.failColor = newColor(defaultFailColor)
	s.warnColor = newColor(defaultWarnColor)

	s.color.EnableColor()
	s.succeedColor.EnableColor()
	s.failColor.EnableColor()
	s.warnColor.EnableColor()

	return s, nil
}

func (s *Spinner) createFrames() {
	f := make([]string, len(s.animation.frames))
	for i, c := range s.animation.frames {
		var symbol = c
		if !s.disableColor || s.color != nil {
			symbol = s.color.SprintfFunc()(c)
		}
		f[i] = fmt.Sprintf("%s %s", symbol, s.message)
	}

	// Set the new animation
	s.frames = f
}

// Start will animate with the recommended speed, this should be the default
// choice.
func (s *Spinner) Start(message string) error {
	return s.StartWithSpeed(message, s.animation.interval)
}

// StartWithSpeed will start animation witha  custom speed for the spinner
func (s *Spinner) StartWithSpeed(message string, speed time.Duration) error {
	s.Lock()
	defer s.Unlock()
	if s.running {
		return errors.New("spinner is already running")
	}

	s.message = message
	s.createFrames()
	s.ticker = time.NewTicker(speed)
	// Start the animation in background
	go func() {
		s.running = true

		for range s.ticker.C {
			s.Render()
		}
	}()
	return nil
}

// Render will render manually an step
func (s *Spinner) Render() error {
	if len(s.frames) == 0 {
		return errors.New("no frames available to to render")
	}

	s.step = s.step % len(s.frames)
	previousLen := len(s.previousFrame)
	s.previousFrame = fmt.Sprintf("%s%s", s.separator, s.frames[s.step])
	newLen := len(s.previousFrame)

	// We need to clean the previous message
	if previousLen > newLen {
		r := previousLen - newLen
		suffix := strings.Repeat(" ", r)
		s.previousFrame = s.previousFrame + suffix
	}

	fmt.Fprint(s.Writer, s.previousFrame)
	s.step++
	return nil
}

// SetMessage will set new message on the animation without stoping it
func (s *Spinner) SetMessage(message string) {
	s.Lock()
	s.message = message
	s.Unlock()
	s.createFrames()
}

// Stop will stop the animation
func (s *Spinner) Stop() error {
	s.Lock()
	defer s.Unlock()
	if !s.running {
		return errors.New("spinner is not running")
	}
	s.ticker.Stop()
	s.running = false
	return nil
}

// Reset will set the spinner to its initial frame
func (s *Spinner) Reset() {
	s.step = 0
	s.createFrames()
}

// Succeed will stop the animation with a success symbol where the spinner is
func (s *Spinner) Succeed() error {
	return s.FinishWithSymbol(s.succeedColor.SprintfFunc()(successSymbol))
}

// Fail will stop the animation with a failure symbol where the spinner is
func (s *Spinner) Fail() error {
	return s.FinishWithSymbol(s.failColor.SprintfFunc()(failureSymbol))
}

// Warn will stop the animation with a warning symbol where the spinner is
func (s *Spinner) Warn() error {
	return s.FinishWithSymbol(s.warnColor.SprintfFunc()(warningSymbol))
}

// Finish will stop an write to the next line
func (s *Spinner) Finish() error {
	if err := s.Stop(); err != nil {
		return err
	}
	s.Reset()
	fmt.Fprint(s.Writer, "\n")
	return nil
}

// FinishWithSymbol will finish the animation with a symbol where the spinner is
func (s *Spinner) FinishWithSymbol(symbol string) error {
	return s.FinishWithMessage(symbol, s.message)
}

// FinishWithMessage will finish animation setting a message and a symbol where the spinner was
func (s *Spinner) FinishWithMessage(symbol, closingMessage string) error {
	if err := s.Stop(); err != nil {
		return err
	}
	s.Reset()
	previousLen := len(s.previousFrame)
	finalMsg := fmt.Sprintf("%s %s", symbol, closingMessage)
	newLen := len(finalMsg)
	if previousLen > newLen {
		r := previousLen - newLen
		suffix := strings.Repeat(" ", r)
		finalMsg = finalMsg + suffix
	}
	fmt.Fprintf(s.Writer, "%s%s\n", s.separator, finalMsg)
	return nil
}
