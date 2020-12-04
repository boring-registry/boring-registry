
# Gospinner [![GoDoc](http://img.shields.io/badge/go-documentation-blue.svg?style=flat-square)](http://godoc.org/github.com/slok/gospinner)  [![Build Status](https://travis-ci.org/slok/gospinner.svg?branch=master)](https://travis-ci.org/slok/gospinner)


Gospinner lets you make simple spinners for your go cli applications. Is fast
and easy to use. Simple customizations and ready to go defaults!



![gospinner](gospinner.gif)


## Install

```
go get github.com/slok/gospinner
```

## Running examples

* [Example 0](https://asciinema.org/a/82944)
* [Example 1](https://asciinema.org/a/82946)
* [Example 2](https://asciinema.org/a/82963)

## Examples


### Default spinner

```go
s, _ := gospinner.NewSpinner(gospinner.Dots)
s.Start("Loading")
// Do stuff
s.Finish()
```

### Custom color spinner

```go
s, _ := gospinner.NewSpinnerWithColor(gospinner.Ball, gospinner.FgGreen)
s.Start("Loading")
// Do stuff
s.Finish()
```

### No color spinner

```go
s, _ := s, _ := gospinner.NewSpinnerNoColor(gospinner.GrowHorizontal)
s.Start("Loading")
// Do stuff
s.Finish()
```

### Spinner with finishers
```go
s, _ := gospinner.NewSpinner(gospinner.Pong)

s.Start("Loading job 1")
// Do job 1 ...
s.Succeed()

s.Start("Loading job 2")
// Do job 2 ...
s.Fail()

s.Start("Loading job 3")
// Do job 3 ...
s.Warn()
```

### Changing message while spinning and finishing with custom message
```go
s, _ := gospinner.NewSpinner(gospinner.Square)
s.Start("Loading")
// Do stuff
s.SetMessage("Starting server dependencies")
// Do more stuff
s.SetMessage("Starting server")
// Do more stuff
s.FinishWithMessage("⚔", "Finished!")
```

### Available spinners:

* Ball
* Column
* Slash
* Square
* Triangle
* Dots
* Dots2
* Pipe
* SimpleDots
* SimpleDotsScrolling
* GrowVertical
* GrowHorizontal
* Arrow
* BouncingBar
* BouncingBall
* Pong
* ProgressBar

For more customizations you should check the [documentation](https://godoc.org/github.com/slok/gospinner)

## Credits

[Xabier Larrakoetxea](http://github.com/slok)


## License

The MIT License (MIT) - see [`LICENSE.md`](LICENSE.md) for more details
