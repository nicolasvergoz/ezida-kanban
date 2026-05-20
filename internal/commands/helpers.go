package commands

import "time"

// nowFunc is the time source used by command run bodies. Tests may
// swap it for a fixed clock if needed.
var nowFunc = time.Now
