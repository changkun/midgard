// Copyright 2020 Changkun Ou. All rights reserved.
// Use of this source code is governed by a GPL-3.0
// license that can be found in the LICENSE file.

package term

// Red turns a given string to red
func Red(in string) string {
	return fgString(in, 255, 0, 0)
}

// Orange turns a given string to orange
func Orange(in string) string {
	return fgString(in, 252, 140, 3)
}

// Green turns a given string to green
func Green(in string) string {
	return fgString(in, 0, 255, 0)
}

// Gray turns a given string to gray
func Gray(in string) string {
	return fgString(in, 125, 125, 125)
}

var (
	before   = []byte("\033[")
	after    = []byte("m")
	reset    = []byte("\033[0;00m")
	fgcolors = fgTermRGB[16:232]
	bgcolors = bgTermRGB[16:232]
)

func fgString(in string, r, g, b uint8) string {
	return string(fgBytes([]byte(in), r, g, b))
}

// Bytes colorizes the foreground with the terminal color that matches
// the closest the RGB color.
func fgBytes(in []byte, r, g, b uint8) []byte {
	return colorize(color(r, g, b, true), in)
}

func colorize(color, in []byte) []byte {
	return append(append(append(append(before, color...), after...), in...), reset...)
}

func color(r, g, b uint8, foreground bool) []byte {
	// if all colors are equal, it might be in the grayscale range
	if r == g && g == b {
		color, ok := grayscale(r, foreground)
		if ok {
			return color
		}
	}

	// the general case approximates RGB by using the closest color.
	r6 := ((uint16(r) * 5) / 255)
	g6 := ((uint16(g) * 5) / 255)
	b6 := ((uint16(b) * 5) / 255)
	i := 36*r6 + 6*g6 + b6
	if foreground {
		return fgcolors[i]
	}
	return bgcolors[i]
}

func grayscale(scale uint8, foreground bool) ([]byte, bool) {
	var source [256][]byte

	if foreground {
		source = fgTermRGB
	} else {
		source = bgTermRGB
	}

	switch scale {
	case 0x08:
		return source[232], true
	case 0x12:
		return source[233], true
	case 0x1c:
		return source[234], true
	case 0x26:
		return source[235], true
	case 0x30:
		return source[236], true
	case 0x3a:
		return source[237], true
	case 0x44:
		return source[238], true
	case 0x4e:
		return source[239], true
	case 0x58:
		return source[240], true
	case 0x62:
		return source[241], true
	case 0x6c:
		return source[242], true
	case 0x76:
		return source[243], true
	case 0x80:
		return source[244], true
	case 0x8a:
		return source[245], true
	case 0x94:
		return source[246], true
	case 0x9e:
		return source[247], true
	case 0xa8:
		return source[248], true
	case 0xb2:
		return source[249], true
	case 0xbc:
		return source[250], true
	case 0xc6:
		return source[251], true
	case 0xd0:
		return source[252], true
	case 0xda:
		return source[253], true
	case 0xe4:
		return source[254], true
	case 0xee:
		return source[255], true
	}
	return nil, false
}

var (
	yellow = fgString("", 252, 140, 3)
	red    = fgString("", 255, 0, 0)
	green  = fgString("", 0, 255, 0)
)

// \033[

var fgTermRGB = [...][]byte{
	[]byte("38;5;0"),
	[]byte("38;5;1"),
	[]byte("38;5;2"),
	[]byte("38;5;3"),
	[]byte("38;5;4"),
	[]byte("38;5;5"),
	[]byte("38;5;6"),
	[]byte("38;5;7"),
	[]byte("38;5;8"),
	[]byte("38;5;9"),
	[]byte("38;5;10"),
	[]byte("38;5;11"),
	[]byte("38;5;12"),
	[]byte("38;5;13"),
	[]byte("38;5;14"),
	[]byte("38;5;15"),
	[]byte("38;5;16"),
	[]byte("38;5;17"),
	[]byte("38;5;18"),
	[]byte("38;5;19"),
	[]byte("38;5;20"),
	[]byte("38;5;21"),
	[]byte("38;5;22"),
	[]byte("38;5;23"),
	[]byte("38;5;24"),
	[]byte("38;5;25"),
	[]byte("38;5;26"),
	[]byte("38;5;27"),
	[]byte("38;5;28"),
	[]byte("38;5;29"),
	[]byte("38;5;30"),
	[]byte("38;5;31"),
	[]byte("38;5;32"),
	[]byte("38;5;33"),
	[]byte("38;5;34"),
	[]byte("38;5;35"),
	[]byte("38;5;36"),
	[]byte("38;5;37"),
	[]byte("38;5;38"),
	[]byte("38;5;39"),
	[]byte("38;5;40"),
	[]byte("38;5;41"),
	[]byte("38;5;42"),
	[]byte("38;5;43"),
	[]byte("38;5;44"),
	[]byte("38;5;45"),
	[]byte("38;5;46"),
	[]byte("38;5;47"),
	[]byte("38;5;48"),
	[]byte("38;5;49"),
	[]byte("38;5;50"),
	[]byte("38;5;51"),
	[]byte("38;5;52"),
	[]byte("38;5;53"),
	[]byte("38;5;54"),
	[]byte("38;5;55"),
	[]byte("38;5;56"),
	[]byte("38;5;57"),
	[]byte("38;5;58"),
	[]byte("38;5;59"),
	[]byte("38;5;60"),
	[]byte("38;5;61"),
	[]byte("38;5;62"),
	[]byte("38;5;63"),
	[]byte("38;5;64"),
	[]byte("38;5;65"),
	[]byte("38;5;66"),
	[]byte("38;5;67"),
	[]byte("38;5;68"),
	[]byte("38;5;69"),
	[]byte("38;5;70"),
	[]byte("38;5;71"),
	[]byte("38;5;72"),
	[]byte("38;5;73"),
	[]byte("38;5;74"),
	[]byte("38;5;75"),
	[]byte("38;5;76"),
	[]byte("38;5;77"),
	[]byte("38;5;78"),
	[]byte("38;5;79"),
	[]byte("38;5;80"),
	[]byte("38;5;81"),
	[]byte("38;5;82"),
	[]byte("38;5;83"),
	[]byte("38;5;84"),
	[]byte("38;5;85"),
	[]byte("38;5;86"),
	[]byte("38;5;87"),
	[]byte("38;5;88"),
	[]byte("38;5;89"),
	[]byte("38;5;90"),
	[]byte("38;5;91"),
	[]byte("38;5;92"),
	[]byte("38;5;93"),
	[]byte("38;5;94"),
	[]byte("38;5;95"),
	[]byte("38;5;96"),
	[]byte("38;5;97"),
	[]byte("38;5;98"),
	[]byte("38;5;99"),
	[]byte("38;5;100"),
	[]byte("38;5;101"),
	[]byte("38;5;102"),
	[]byte("38;5;103"),
	[]byte("38;5;104"),
	[]byte("38;5;105"),
	[]byte("38;5;106"),
	[]byte("38;5;107"),
	[]byte("38;5;108"),
	[]byte("38;5;109"),
	[]byte("38;5;110"),
	[]byte("38;5;111"),
	[]byte("38;5;112"),
	[]byte("38;5;113"),
	[]byte("38;5;114"),
	[]byte("38;5;115"),
	[]byte("38;5;116"),
	[]byte("38;5;117"),
	[]byte("38;5;118"),
	[]byte("38;5;119"),
	[]byte("38;5;120"),
	[]byte("38;5;121"),
	[]byte("38;5;122"),
	[]byte("38;5;123"),
	[]byte("38;5;124"),
	[]byte("38;5;125"),
	[]byte("38;5;126"),
	[]byte("38;5;127"),
	[]byte("38;5;128"),
	[]byte("38;5;129"),
	[]byte("38;5;130"),
	[]byte("38;5;131"),
	[]byte("38;5;132"),
	[]byte("38;5;133"),
	[]byte("38;5;134"),
	[]byte("38;5;135"),
	[]byte("38;5;136"),
	[]byte("38;5;137"),
	[]byte("38;5;138"),
	[]byte("38;5;139"),
	[]byte("38;5;140"),
	[]byte("38;5;141"),
	[]byte("38;5;142"),
	[]byte("38;5;143"),
	[]byte("38;5;144"),
	[]byte("38;5;145"),
	[]byte("38;5;146"),
	[]byte("38;5;147"),
	[]byte("38;5;148"),
	[]byte("38;5;149"),
	[]byte("38;5;150"),
	[]byte("38;5;151"),
	[]byte("38;5;152"),
	[]byte("38;5;153"),
	[]byte("38;5;154"),
	[]byte("38;5;155"),
	[]byte("38;5;156"),
	[]byte("38;5;157"),
	[]byte("38;5;158"),
	[]byte("38;5;159"),
	[]byte("38;5;160"),
	[]byte("38;5;161"),
	[]byte("38;5;162"),
	[]byte("38;5;163"),
	[]byte("38;5;164"),
	[]byte("38;5;165"),
	[]byte("38;5;166"),
	[]byte("38;5;167"),
	[]byte("38;5;168"),
	[]byte("38;5;169"),
	[]byte("38;5;170"),
	[]byte("38;5;171"),
	[]byte("38;5;172"),
	[]byte("38;5;173"),
	[]byte("38;5;174"),
	[]byte("38;5;175"),
	[]byte("38;5;176"),
	[]byte("38;5;177"),
	[]byte("38;5;178"),
	[]byte("38;5;179"),
	[]byte("38;5;180"),
	[]byte("38;5;181"),
	[]byte("38;5;182"),
	[]byte("38;5;183"),
	[]byte("38;5;184"),
	[]byte("38;5;185"),
	[]byte("38;5;186"),
	[]byte("38;5;187"),
	[]byte("38;5;188"),
	[]byte("38;5;189"),
	[]byte("38;5;190"),
	[]byte("38;5;191"),
	[]byte("38;5;192"),
	[]byte("38;5;193"),
	[]byte("38;5;194"),
	[]byte("38;5;195"),
	[]byte("38;5;196"),
	[]byte("38;5;197"),
	[]byte("38;5;198"),
	[]byte("38;5;199"),
	[]byte("38;5;200"),
	[]byte("38;5;201"),
	[]byte("38;5;202"),
	[]byte("38;5;203"),
	[]byte("38;5;204"),
	[]byte("38;5;205"),
	[]byte("38;5;206"),
	[]byte("38;5;207"),
	[]byte("38;5;208"),
	[]byte("38;5;209"),
	[]byte("38;5;210"),
	[]byte("38;5;211"),
	[]byte("38;5;212"),
	[]byte("38;5;213"),
	[]byte("38;5;214"),
	[]byte("38;5;215"),
	[]byte("38;5;216"),
	[]byte("38;5;217"),
	[]byte("38;5;218"),
	[]byte("38;5;219"),
	[]byte("38;5;220"),
	[]byte("38;5;221"),
	[]byte("38;5;222"),
	[]byte("38;5;223"),
	[]byte("38;5;224"),
	[]byte("38;5;225"),
	[]byte("38;5;226"),
	[]byte("38;5;227"),
	[]byte("38;5;228"),
	[]byte("38;5;229"),
	[]byte("38;5;230"),
	[]byte("38;5;231"),
	[]byte("38;5;232"),
	[]byte("38;5;233"),
	[]byte("38;5;234"),
	[]byte("38;5;235"),
	[]byte("38;5;236"),
	[]byte("38;5;237"),
	[]byte("38;5;238"),
	[]byte("38;5;239"),
	[]byte("38;5;240"),
	[]byte("38;5;241"),
	[]byte("38;5;242"),
	[]byte("38;5;243"),
	[]byte("38;5;244"),
	[]byte("38;5;245"),
	[]byte("38;5;246"),
	[]byte("38;5;247"),
	[]byte("38;5;248"),
	[]byte("38;5;249"),
	[]byte("38;5;250"),
	[]byte("38;5;251"),
	[]byte("38;5;252"),
	[]byte("38;5;253"),
	[]byte("38;5;254"),
	[]byte("38;5;255"),
}

var bgTermRGB = [...][]byte{
	[]byte("48;5;0"),
	[]byte("48;5;1"),
	[]byte("48;5;2"),
	[]byte("48;5;3"),
	[]byte("48;5;4"),
	[]byte("48;5;5"),
	[]byte("48;5;6"),
	[]byte("48;5;7"),
	[]byte("48;5;8"),
	[]byte("48;5;9"),
	[]byte("48;5;10"),
	[]byte("48;5;11"),
	[]byte("48;5;12"),
	[]byte("48;5;13"),
	[]byte("48;5;14"),
	[]byte("48;5;15"),
	[]byte("48;5;16"),
	[]byte("48;5;17"),
	[]byte("48;5;18"),
	[]byte("48;5;19"),
	[]byte("48;5;20"),
	[]byte("48;5;21"),
	[]byte("48;5;22"),
	[]byte("48;5;23"),
	[]byte("48;5;24"),
	[]byte("48;5;25"),
	[]byte("48;5;26"),
	[]byte("48;5;27"),
	[]byte("48;5;28"),
	[]byte("48;5;29"),
	[]byte("48;5;30"),
	[]byte("48;5;31"),
	[]byte("48;5;32"),
	[]byte("48;5;33"),
	[]byte("48;5;34"),
	[]byte("48;5;35"),
	[]byte("48;5;36"),
	[]byte("48;5;37"),
	[]byte("48;5;38"),
	[]byte("48;5;39"),
	[]byte("48;5;40"),
	[]byte("48;5;41"),
	[]byte("48;5;42"),
	[]byte("48;5;43"),
	[]byte("48;5;44"),
	[]byte("48;5;45"),
	[]byte("48;5;46"),
	[]byte("48;5;47"),
	[]byte("48;5;48"),
	[]byte("48;5;49"),
	[]byte("48;5;50"),
	[]byte("48;5;51"),
	[]byte("48;5;52"),
	[]byte("48;5;53"),
	[]byte("48;5;54"),
	[]byte("48;5;55"),
	[]byte("48;5;56"),
	[]byte("48;5;57"),
	[]byte("48;5;58"),
	[]byte("48;5;59"),
	[]byte("48;5;60"),
	[]byte("48;5;61"),
	[]byte("48;5;62"),
	[]byte("48;5;63"),
	[]byte("48;5;64"),
	[]byte("48;5;65"),
	[]byte("48;5;66"),
	[]byte("48;5;67"),
	[]byte("48;5;68"),
	[]byte("48;5;69"),
	[]byte("48;5;70"),
	[]byte("48;5;71"),
	[]byte("48;5;72"),
	[]byte("48;5;73"),
	[]byte("48;5;74"),
	[]byte("48;5;75"),
	[]byte("48;5;76"),
	[]byte("48;5;77"),
	[]byte("48;5;78"),
	[]byte("48;5;79"),
	[]byte("48;5;80"),
	[]byte("48;5;81"),
	[]byte("48;5;82"),
	[]byte("48;5;83"),
	[]byte("48;5;84"),
	[]byte("48;5;85"),
	[]byte("48;5;86"),
	[]byte("48;5;87"),
	[]byte("48;5;88"),
	[]byte("48;5;89"),
	[]byte("48;5;90"),
	[]byte("48;5;91"),
	[]byte("48;5;92"),
	[]byte("48;5;93"),
	[]byte("48;5;94"),
	[]byte("48;5;95"),
	[]byte("48;5;96"),
	[]byte("48;5;97"),
	[]byte("48;5;98"),
	[]byte("48;5;99"),
	[]byte("48;5;100"),
	[]byte("48;5;101"),
	[]byte("48;5;102"),
	[]byte("48;5;103"),
	[]byte("48;5;104"),
	[]byte("48;5;105"),
	[]byte("48;5;106"),
	[]byte("48;5;107"),
	[]byte("48;5;108"),
	[]byte("48;5;109"),
	[]byte("48;5;110"),
	[]byte("48;5;111"),
	[]byte("48;5;112"),
	[]byte("48;5;113"),
	[]byte("48;5;114"),
	[]byte("48;5;115"),
	[]byte("48;5;116"),
	[]byte("48;5;117"),
	[]byte("48;5;118"),
	[]byte("48;5;119"),
	[]byte("48;5;120"),
	[]byte("48;5;121"),
	[]byte("48;5;122"),
	[]byte("48;5;123"),
	[]byte("48;5;124"),
	[]byte("48;5;125"),
	[]byte("48;5;126"),
	[]byte("48;5;127"),
	[]byte("48;5;128"),
	[]byte("48;5;129"),
	[]byte("48;5;130"),
	[]byte("48;5;131"),
	[]byte("48;5;132"),
	[]byte("48;5;133"),
	[]byte("48;5;134"),
	[]byte("48;5;135"),
	[]byte("48;5;136"),
	[]byte("48;5;137"),
	[]byte("48;5;138"),
	[]byte("48;5;139"),
	[]byte("48;5;140"),
	[]byte("48;5;141"),
	[]byte("48;5;142"),
	[]byte("48;5;143"),
	[]byte("48;5;144"),
	[]byte("48;5;145"),
	[]byte("48;5;146"),
	[]byte("48;5;147"),
	[]byte("48;5;148"),
	[]byte("48;5;149"),
	[]byte("48;5;150"),
	[]byte("48;5;151"),
	[]byte("48;5;152"),
	[]byte("48;5;153"),
	[]byte("48;5;154"),
	[]byte("48;5;155"),
	[]byte("48;5;156"),
	[]byte("48;5;157"),
	[]byte("48;5;158"),
	[]byte("48;5;159"),
	[]byte("48;5;160"),
	[]byte("48;5;161"),
	[]byte("48;5;162"),
	[]byte("48;5;163"),
	[]byte("48;5;164"),
	[]byte("48;5;165"),
	[]byte("48;5;166"),
	[]byte("48;5;167"),
	[]byte("48;5;168"),
	[]byte("48;5;169"),
	[]byte("48;5;170"),
	[]byte("48;5;171"),
	[]byte("48;5;172"),
	[]byte("48;5;173"),
	[]byte("48;5;174"),
	[]byte("48;5;175"),
	[]byte("48;5;176"),
	[]byte("48;5;177"),
	[]byte("48;5;178"),
	[]byte("48;5;179"),
	[]byte("48;5;180"),
	[]byte("48;5;181"),
	[]byte("48;5;182"),
	[]byte("48;5;183"),
	[]byte("48;5;184"),
	[]byte("48;5;185"),
	[]byte("48;5;186"),
	[]byte("48;5;187"),
	[]byte("48;5;188"),
	[]byte("48;5;189"),
	[]byte("48;5;190"),
	[]byte("48;5;191"),
	[]byte("48;5;192"),
	[]byte("48;5;193"),
	[]byte("48;5;194"),
	[]byte("48;5;195"),
	[]byte("48;5;196"),
	[]byte("48;5;197"),
	[]byte("48;5;198"),
	[]byte("48;5;199"),
	[]byte("48;5;200"),
	[]byte("48;5;201"),
	[]byte("48;5;202"),
	[]byte("48;5;203"),
	[]byte("48;5;204"),
	[]byte("48;5;205"),
	[]byte("48;5;206"),
	[]byte("48;5;207"),
	[]byte("48;5;208"),
	[]byte("48;5;209"),
	[]byte("48;5;210"),
	[]byte("48;5;211"),
	[]byte("48;5;212"),
	[]byte("48;5;213"),
	[]byte("48;5;214"),
	[]byte("48;5;215"),
	[]byte("48;5;216"),
	[]byte("48;5;217"),
	[]byte("48;5;218"),
	[]byte("48;5;219"),
	[]byte("48;5;220"),
	[]byte("48;5;221"),
	[]byte("48;5;222"),
	[]byte("48;5;223"),
	[]byte("48;5;224"),
	[]byte("48;5;225"),
	[]byte("48;5;226"),
	[]byte("48;5;227"),
	[]byte("48;5;228"),
	[]byte("48;5;229"),
	[]byte("48;5;230"),
	[]byte("48;5;231"),
	[]byte("48;5;232"),
	[]byte("48;5;233"),
	[]byte("48;5;234"),
	[]byte("48;5;235"),
	[]byte("48;5;236"),
	[]byte("48;5;237"),
	[]byte("48;5;238"),
	[]byte("48;5;239"),
	[]byte("48;5;240"),
	[]byte("48;5;241"),
	[]byte("48;5;242"),
	[]byte("48;5;243"),
	[]byte("48;5;244"),
	[]byte("48;5;245"),
	[]byte("48;5;246"),
	[]byte("48;5;247"),
	[]byte("48;5;248"),
	[]byte("48;5;249"),
	[]byte("48;5;250"),
	[]byte("48;5;251"),
	[]byte("48;5;252"),
	[]byte("48;5;253"),
	[]byte("48;5;254"),
	[]byte("48;5;255"),
}
