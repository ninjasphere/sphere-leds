package leds

import (
	"fmt"
	"os"
	"sync"
	"time"
)

const pwmPeriod = 1000000

var Colors = map[string][]int{
	"black":   {0, 0, 0},
	"red":     {1, 0, 0},
	"green":   {0, 1, 0},
	"blue":    {0, 0, 1},
	"cyan":    {0, 1, 1},
	"magenta": {1, 0, 1},
	"yellow":  {1, 1, 0},
	"white":   {1, 1, 1},
}

var LedNames = []string{
	"power",
	"wired_internet",
	"wireless",
	"pairing",
	"radio",
}

var LedPositions = [][]int{
	{15, 13, 14},
	{12, 10, 11},
	{9, 1, 8},
	{2, 4, 3},
	{5, 7, 6},
}

var is3_2 = false

// holds the state for an array of leds on our board.
type LedArray struct {
	Leds         []int
	LedStates    []LedState
	BlinkOnState bool
	ticker       *time.Ticker
	lock         *sync.Mutex
	flashDirty   bool // true if the background thread needs to check the flash state on next cycle, false otherwise
}

type LedState struct {
	Flash bool
	Color string
	On    bool
}

func CreateLedArray() *LedArray {
	ledArr := &LedArray{
		Leds:       []int{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0},
		LedStates:  make([]LedState, 5),
		lock:       &sync.Mutex{},
		flashDirty: true,
	}
	initLEDs()
	go ledArr.setupBackgroundJob()

	return ledArr
}

func (l *LedArray) setupBackgroundJob() {
	l.ticker = time.NewTicker(1 * time.Second)
	for {
		select {
		case <-l.ticker.C:

			l.lock.Lock()

			//log.Println("[DEBUG] flash")
			if l.flashDirty {

				flashCount := 0

				for n := range l.LedStates {
					if l.LedStates[n].Flash {
						flashCount += 1

						if l.LedStates[n].On {
							l.setColorInt(n, Colors["black"])
							l.LedStates[n].On = false
							l.BlinkOnState = false
						} else {
							l.setColorInt(n, Colors[l.LedStates[n].Color])
							l.LedStates[n].On = true
							l.BlinkOnState = true
						}
					}
				}

				l.SetLEDs()

				l.flashDirty = (flashCount > 0)
			}

			l.lock.Unlock()

		}
	}
}

func (l *LedArray) setColorInt(position int, color []int) {
	var indexes = LedPositions[position]
	for i := 0; i < 3; i++ {
		l.Leds[indexes[i]] = color[i]
	}
}

func (l *LedArray) SetPwmBrightness(brightness int) {
	if is3_2 {
		writetofile("/sys/class/pwm/ehrpwm.0:0/run/duty_percent", fmt.Sprintf("%d", brightness))
	} else {

		var normalized int

		if brightness < 0 {
			normalized = 0
		} else if normalized > brightness {
			normalized = pwmPeriod
		} else {
			normalized = brightness * pwmPeriod / 100
		}
		writetofile("/sys/class/pwm/pwmchip0/pwm0/duty_cycle", fmt.Sprintf("%d", normalized))
		writetofile("/sys/class/pwm/pwmchip0/pwm0/enable", "1")
	}
}

func (l *LedArray) SetColor(position int, color string, flash bool) {
	defer l.lock.Unlock()
	l.lock.Lock()
	// update the state
	l.LedStates[position].Flash = flash
	l.LedStates[position].Color = color
	if flash {
		l.LedStates[position].On = l.BlinkOnState
	} else {
		l.LedStates[position].On = true
	}
	l.setColorInt(position, Colors[color])
	if flash {
		l.flashDirty = true
	}
	// apply it
	l.SetLEDs()
}

func (l *LedArray) Reset() {
	defer l.lock.Unlock()
	l.lock.Lock()
	for pos := range LedNames {
		// update the states
		l.LedStates[pos].Flash = false
		l.LedStates[pos].Color = "black"
		l.LedStates[pos].On = true
		l.setColorInt(pos, Colors["black"])
	}
	l.flashDirty = true
	// apply it
	l.SetLEDs()
}

func ValidBrightness(brightness int) bool {
	return brightness >= 0 && brightness <= 100
}

func ValidColor(color string) bool {
	return Colors[color] != nil
}

func ValidLedName(name string) bool {
	for n := range LedNames {
		if LedNames[n] == name {
			return true
		}
	}
	return false
}

func LedNameIndex(name string) int {
	for n := range LedNames {
		if LedNames[n] == name {
			return n
		}
	}
	panic("LedName didn't exist.")
}

func initLEDs() {
	// underlight
	if is3_2 {
		writetofile("/sys/kernel/debug/omap_mux/spi0_sclk", "03")
	}

	// echo 0   > run stop
	// echo 0   > duty_percent make it black
	// echo 200 > period_freq oscilating frequency
	// echo 1   > run run it
	if is3_2 {
		writetofile("/sys/class/pwm/ehrpwm.0:0/run", "0")
		writetofile("/sys/class/pwm/ehrpwm.0:0/period_freq", "200")
		writetofile("/sys/class/pwm/ehrpwm.0:0/run", "1")
	} else {
		writetofile("/sys/class/pwm/pwmchip0/export", "0")
		writetofile("/sys/class/pwm/pwmchip0/pwm0/enable", "0")
		writetofile("/sys/class/pwm/pwmchip0/pwm0/period", fmt.Sprintf("%d", pwmPeriod))
	}

	if is3_2 {
		writetofile("/sys/kernel/debug/omap_mux/lcd_data15", "27")
		writetofile("/sys/kernel/debug/omap_mux/lcd_data14", "27")
		writetofile("/sys/kernel/debug/omap_mux/uart0_ctsn", "27")
		writetofile("/sys/kernel/debug/omap_mux/mii1_col", "27")
	}

	if _, err := os.Stat("/sys/class/gpio/gpio11/direction"); os.IsNotExist(err) {
		writetofile("/sys/class/gpio/export", "11")
	}

	if _, err := os.Stat("/sys/class/gpio/gpio10/direction"); os.IsNotExist(err) {
		writetofile("/sys/class/gpio/export", "10")
	}

	if _, err := os.Stat("/sys/class/gpio/gpio40/direction"); os.IsNotExist(err) {
		writetofile("/sys/class/gpio/export", "40")
	}

	if _, err := os.Stat("/sys/class/gpio/gpio96/direction"); os.IsNotExist(err) {
		writetofile("/sys/class/gpio/export", "96")
	}

	writetofile("/sys/class/gpio/gpio11/direction", "low")
	writetofile("/sys/class/gpio/gpio10/direction", "low")
	writetofile("/sys/class/gpio/gpio40/direction", "low")
	writetofile("/sys/class/gpio/gpio96/direction", "low")

}

func (l *LedArray) SetLEDs() {
	//	log.Printf("[DEBUG] Updating leds: %v", l.Leds)
	//	log.Printf("[DEBUG] Updating flashstate: %v", l.LedStates)

	for i := range l.Leds {
		writetofile("/sys/class/gpio/gpio40/value", fmt.Sprintf("%d", l.Leds[i]))
		writetofile("/sys/class/gpio/gpio96/value", "1")
		writetofile("/sys/class/gpio/gpio96/value", "0")
	}

	writetofile("/sys/class/gpio/gpio11/value", "1")
	writetofile("/sys/class/gpio/gpio11/value", "0")
}

func writetofile(fn string, val string) error {

	df, err := os.OpenFile(fn, os.O_WRONLY|os.O_SYNC, 0666)

	if err != nil {
		return err
	}

	defer df.Close()

	if _, err = fmt.Fprintln(df, val); err != nil {
		return err
	}

	return nil
}
