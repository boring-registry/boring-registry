/*
Package gospinner is a spinner package to create spinners in a fast, beatiful
and easy way. No more ugly and old cli applications, give feedback to your users

Use the simplest spinner and start getting awsome cli applications:

    s, _ := gospinner.NewSpinnerWithColor(gospinner.Ball, gospinner.FgGreen)
    s.Start("Loading")
    // Do stuff
    s.Finish()

However sometimes you want custom colors for your spinners:

    s, _ := s, _ := gospinner.NewSpinnerNoColor(gospinner.GrowHorizontal)
    s.Start("Loading")
    // Do stuff
    s.Finish()

Easy right? but that finish line is a little bit sad, Lets use some cool finishers!

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

You have more stuff to customize it, check it on the documentation.
*/
package gospinner
