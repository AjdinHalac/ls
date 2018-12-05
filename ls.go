package main

import (
	"bufio"
	"bytes"
	"fmt"
	"golang.org/x/crypto/ssh/terminal"
	"io/ioutil"
	"math"
	"os"
	"os/user"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// Base set of color codes for colorized output
const (
	colorFgBlack   = 30
	colorFgRed     = 31
	colorFgGreen   = 32
	colorFgBrown   = 33
	colorFgBlue    = 34
	colorFgMagenta = 35
	colorFgCyan    = 36
	colorFgWhite   = 37
	colorBgBlack   = 40
	colorBgRed     = 41
	colorBgGreen   = 42
	colorBgBrown   = 43
	colorBgBlue    = 44
	colorBgMagenta = 45
	colorBgCyan    = 46
	colorBgWhite   = 47
)

// This a FileInfo paired with the original path as passed in to the program.
// Unfortunately, the Name() in FileInfo is only the basename, so the associated
// path must be manually recorded as well.
type FileInfoPath struct {
	path string
	info os.FileInfo
}

// This struct wraps all the option settings for the program into a single
// object.
type Options struct {
	all         bool
	long        bool
	human       bool
	one         bool
	dir         bool
	color       bool
	sortReverse bool
	sortTime    bool
	sortSize    bool
	help        bool
	dirsFirst   bool
}

// Listings contain all the information about a file or directory in a printable
// form.
type Listing struct {
	permissions  string
	numHardLinks string
	owner        string
	group        string
	size         string
	epochNano    int64
	month        string
	day          string
	time         string
	name         string
	linkName     string
	linkOrphan   bool
	isSocket     bool
	isPipe       bool
	isBlock      bool
	isCharacter  bool
}

// Global variables used by multiple functions
var (
	userMap  map[int]string    // matches uid to username
	groupMap map[int]string    // matches gid to groupname
	colorMap map[string]string // matches file specification to output color
	options  Options           // the state of all program options
)

// Helper function for get_color_from_bsd_code.  Given a flag to indicate
// foreground/background and a single letter, return the correct partial ASCII
// color code.
func getPartialColor(foreground bool, letter uint8) string {
	var partialBytes bytes.Buffer

	if foreground && letter == 'x' {
		partialBytes.WriteString("0;")
	} else if !foreground && letter != 'x' {
		partialBytes.WriteString(";")
	}

	if foreground && letter >= 97 && letter <= 122 {
		partialBytes.WriteString("0;")
	} else if foreground && letter >= 65 && letter <= 90 {
		partialBytes.WriteString("1;")
	}

	if letter == 'a' {
		if foreground {
			partialBytes.WriteString(strconv.Itoa(colorFgBlack))
		} else if !foreground {
			partialBytes.WriteString(strconv.Itoa(colorBgBlack))
		}
	} else if letter == 'b' {
		if foreground {
			partialBytes.WriteString(strconv.Itoa(colorFgRed))
		} else if !foreground {
			partialBytes.WriteString(strconv.Itoa(colorBgRed))
		}
	} else if letter == 'c' {
		if foreground {
			partialBytes.WriteString(strconv.Itoa(colorFgGreen))
		} else if !foreground {
			partialBytes.WriteString(strconv.Itoa(colorBgGreen))
		}
	} else if letter == 'd' {
		if foreground {
			partialBytes.WriteString(strconv.Itoa(colorFgBrown))
		} else if !foreground {
			partialBytes.WriteString(strconv.Itoa(colorBgBrown))
		}
	} else if letter == 'e' {
		if foreground {
			partialBytes.WriteString(strconv.Itoa(colorFgBlue))
		} else if !foreground {
			partialBytes.WriteString(strconv.Itoa(colorBgBlue))
		}
	} else if letter == 'f' {
		if foreground {
			partialBytes.WriteString(strconv.Itoa(colorFgMagenta))
		} else if !foreground {
			partialBytes.WriteString(strconv.Itoa(colorBgMagenta))
		}
	} else if letter == 'g' {
		if foreground {
			partialBytes.WriteString(strconv.Itoa(colorFgCyan))
		} else if !foreground {
			partialBytes.WriteString(strconv.Itoa(colorBgCyan))
		}
	} else if letter == 'h' {
		if foreground {
			partialBytes.WriteString(strconv.Itoa(colorFgWhite))
		} else if !foreground {
			partialBytes.WriteString(strconv.Itoa(colorBgWhite))
		}
	} else if letter == 'A' {
		partialBytes.WriteString(strconv.Itoa(colorFgBlack))
	} else if letter == 'B' {
		partialBytes.WriteString(strconv.Itoa(colorFgRed))
	} else if letter == 'C' {
		partialBytes.WriteString(strconv.Itoa(colorFgGreen))
	} else if letter == 'D' {
		partialBytes.WriteString(strconv.Itoa(colorFgBrown))
	} else if letter == 'E' {
		partialBytes.WriteString(strconv.Itoa(colorFgBlue))
	} else if letter == 'F' {
		partialBytes.WriteString(strconv.Itoa(colorFgMagenta))
	} else if letter == 'G' {
		partialBytes.WriteString(strconv.Itoa(colorFgCyan))
	} else if letter == 'H' {
		partialBytes.WriteString(strconv.Itoa(colorFgWhite))
	}

	return partialBytes.String()
}

// Given a BSD LSCOLORS code like "ex", return the proper ASCII code
// (like "\x1b[0;32m")
func getColorFromBsdCode(code string) string {
	colorForeground := code[0]
	colorBackground := code[1]

	var colorBytes bytes.Buffer
	colorBytes.WriteString("\x1b[")
	colorBytes.WriteString(getPartialColor(true, colorForeground))
	colorBytes.WriteString(getPartialColor(false, colorBackground))
	colorBytes.WriteString("m")

	return colorBytes.String()
}

// Given an LSCOLORS string, fill in the appropriate keys and values of the
// global color_map.
func parseLscolors(LSCOLORS string) {
	for i := 0; i < len(LSCOLORS); i += 2 {
		if i == 0 {
			colorMap["directory"] =
				getColorFromBsdCode(LSCOLORS[i : i+2])
		} else if i == 2 {
			colorMap["symlink"] =
				getColorFromBsdCode(LSCOLORS[i : i+2])
		} else if i == 4 {
			colorMap["socket"] =
				getColorFromBsdCode(LSCOLORS[i : i+2])
		} else if i == 6 {
			colorMap["pipe"] =
				getColorFromBsdCode(LSCOLORS[i : i+2])
		} else if i == 8 {
			colorMap["executable"] =
				getColorFromBsdCode(LSCOLORS[i : i+2])
		} else if i == 10 {
			colorMap["block"] =
				getColorFromBsdCode(LSCOLORS[i : i+2])
		} else if i == 12 {
			colorMap["character"] =
				getColorFromBsdCode(LSCOLORS[i : i+2])
		} else if i == 14 {
			colorMap["executable_suid"] =
				getColorFromBsdCode(LSCOLORS[i : i+2])
		} else if i == 16 {
			colorMap["executable_sgid"] =
				getColorFromBsdCode(LSCOLORS[i : i+2])
		} else if i == 18 {
			colorMap["directory_o+w_sticky"] =
				getColorFromBsdCode(LSCOLORS[i : i+2])
		} else if i == 20 {
			colorMap["directory_o+w"] =
				getColorFromBsdCode(LSCOLORS[i : i+2])
		}
	}
}

// Write the given Listing's name to the output buffer, with the appropriate
// formatting based on the current options.
func writeListingName(outputBuffer *bytes.Buffer, l Listing) {

	if options.color {
		appliedColor := false

		numHardlinks, _ := strconv.Atoi(l.numHardLinks)

		// "file.name.txt" -> "*.txt"
		nameSplit := strings.Split(l.name, ".")
		extensionStr := ""
		if len(nameSplit) > 1 {
			extensionStr = fmt.Sprintf("*.%s", nameSplit[len(nameSplit)-1])
		}

		if extensionStr != "" && colorMap[extensionStr] != "" {
			outputBuffer.WriteString(colorMap[extensionStr])
			appliedColor = true
		} else if l.permissions[0] == 'd' &&
			l.permissions[8] == 'w' && l.permissions[9] == 't' {
			outputBuffer.WriteString(colorMap["directory_o+w_sticky"])
			appliedColor = true
		} else if l.permissions[0] == 'd' && l.permissions[9] == 't' {
			outputBuffer.WriteString(colorMap["directory_sticky"])
			appliedColor = true
		} else if l.permissions[0] == 'd' && l.permissions[8] == 'w' {
			outputBuffer.WriteString(colorMap["directory_o+w"])
			appliedColor = true
		} else if l.permissions[0] == 'd' { // directory
			outputBuffer.WriteString(colorMap["directory"])
			appliedColor = true
		} else if numHardlinks > 1 { // multiple hardlinks
			outputBuffer.WriteString(colorMap["multi_hardlink"])
			appliedColor = true
		} else if l.permissions[0] == 'l' && l.linkOrphan { // orphan link
			outputBuffer.WriteString(colorMap["link_orphan"])
			appliedColor = true
		} else if l.permissions[0] == 'l' { // symlink
			outputBuffer.WriteString(colorMap["symlink"])
			appliedColor = true
		} else if l.permissions[3] == 's' { // setuid
			outputBuffer.WriteString(colorMap["executable_suid"])
			appliedColor = true
		} else if l.permissions[6] == 's' { // setgid
			outputBuffer.WriteString(colorMap["executable_sgid"])
			appliedColor = true
		} else if strings.Contains(l.permissions, "x") { // executable
			outputBuffer.WriteString(colorMap["executable"])
			appliedColor = true
		} else if l.isSocket { // socket
			outputBuffer.WriteString(colorMap["socket"])
			appliedColor = true
		} else if l.isPipe { // pipe
			outputBuffer.WriteString(colorMap["pipe"])
			appliedColor = true
		} else if l.isBlock { // block
			outputBuffer.WriteString(colorMap["block"])
			appliedColor = true
		} else if l.isCharacter { // character
			outputBuffer.WriteString(colorMap["character"])
			appliedColor = true
		}

		outputBuffer.WriteString(l.name)
		if appliedColor {
			outputBuffer.WriteString(colorMap["end"])
		}
	} else {
		outputBuffer.WriteString(l.name)
	}

	if l.permissions[0] == 'l' && options.long {
		if l.linkOrphan {
			outputBuffer.WriteString(fmt.Sprintf(" -> %s%s%s",
				colorMap["link_orphan_target"],
				l.linkName,
				colorMap["end"]))
		} else {
			outputBuffer.WriteString(fmt.Sprintf(" -> %s", l.linkName))
		}
	}
}

// Convert a FileInfoPath object to a Listing.  The dirname is passed for
// following symlinks.
func createListing(dirname string, fip FileInfoPath) (Listing, error) {
	var currentListing Listing

	// permissions string
	currentListing.permissions = fip.info.Mode().String()
	if fip.info.Mode()&os.ModeSymlink == os.ModeSymlink {
		currentListing.permissions = strings.Replace(
			currentListing.permissions, "L", "l", 1)

		var _pathstr string
		if dirname == "" {
			_pathstr = fmt.Sprintf("%s", fip.path)
		} else {
			_pathstr = fmt.Sprintf("%s/%s", dirname, fip.path)
		}
		link, err := os.Readlink(fmt.Sprintf(_pathstr))
		if err != nil {
			return currentListing, err
		}
		currentListing.linkName = link

		// check to see if the symlink target exists
		var linkPathstr string
		if dirname == "" {
			linkPathstr = fmt.Sprintf("%s", link)
		} else {
			linkPathstr = fmt.Sprintf("%s/%s", dirname, link)
		}
		_, err = os.Open(linkPathstr)
		if err != nil {
			if os.IsNotExist(err) {
				currentListing.linkOrphan = true
			} else {
				return currentListing, err
			}
		}
	} else if currentListing.permissions[0] == 'D' {
		currentListing.permissions = currentListing.permissions[1:]
	} else if currentListing.permissions[0:2] == "ug" {
		currentListing.permissions =
			strings.Replace(currentListing.permissions, "ug", "-", 1)
		currentListing.permissions = fmt.Sprintf("%ss%ss%s",
			currentListing.permissions[0:3],
			currentListing.permissions[4:6],
			currentListing.permissions[7:])
	} else if currentListing.permissions[0] == 'u' {
		currentListing.permissions =
			strings.Replace(currentListing.permissions, "u", "-", 1)
		currentListing.permissions = fmt.Sprintf("%ss%s",
			currentListing.permissions[0:3],
			currentListing.permissions[4:])
	} else if currentListing.permissions[0] == 'g' {
		currentListing.permissions =
			strings.Replace(currentListing.permissions, "g", "-", 1)
		currentListing.permissions = fmt.Sprintf("%ss%s",
			currentListing.permissions[0:6],
			currentListing.permissions[7:])
	} else if currentListing.permissions[0:2] == "dt" {
		currentListing.permissions =
			strings.Replace(currentListing.permissions, "dt", "d", 1)
		currentListing.permissions = fmt.Sprintf("%st",
			currentListing.permissions[0:len(currentListing.permissions)-1])
	}

	sys := fip.info.Sys()

	stat, ok := sys.(*syscall.Stat_t)
	if !ok {
		return currentListing, fmt.Errorf("syscall failed\n")
	}

	// number of hard links
	numHardLinks := uint64(stat.Nlink)
	currentListing.numHardLinks = fmt.Sprintf("%d", numHardLinks)

	// owner
	owner, err := user.LookupId(fmt.Sprintf("%d", stat.Uid))
	if err != nil {
		// if this causes an error, use the manual user_map
		//
		// this can happen if go is built using cross-compilation for multiple
		// architectures (such as with Fedora Linux), in which case these
		// OS-specific features aren't implemented
		_owner := userMap[int(stat.Uid)]
		if _owner == "" {
			// if the user isn't in the map, just use the uid number
			currentListing.owner = fmt.Sprintf("%d", stat.Uid)
		} else {
			currentListing.owner = _owner
		}
	} else {
		currentListing.owner = owner.Username
	}

	// group
	_group := groupMap[int(stat.Gid)]
	if _group == "" {
		// if the group isn't in the map, just use the gid number
		currentListing.group = fmt.Sprintf("%d", stat.Gid)
	} else {
		currentListing.group = _group
	}

	// size
	if options.human {
		size := float64(fip.info.Size())

		count := 0
		for size >= 1.0 {
			size /= 1024
			count++
		}

		if count < 0 {
			count = 0
		} else if count > 0 {
			size *= 1024
			count--
		}

		var suffix string
		if count == 0 {
			suffix = "B"
		} else if count == 1 {
			suffix = "K"
		} else if count == 2 {
			suffix = "M"
		} else if count == 3 {
			suffix = "G"
		} else if count == 4 {
			suffix = "T"
		} else if count == 5 {
			suffix = "P"
		} else if count == 6 {
			suffix = "E"
		} else {
			suffix = "?"
		}

		sizeStr := ""
		if count == 0 {
			sizeB := int64(size)
			sizeStr = fmt.Sprintf("%d%s", sizeB, suffix)
		} else {
			// looks like the printf formatting automatically rounds up
			sizeStr = fmt.Sprintf("%.1f%s", size, suffix)
		}

		// drop the trailing .0 if it exists in the size
		// e.g. 14.0K -> 14K
		if len(sizeStr) > 3 &&
			sizeStr[len(sizeStr)-3:len(sizeStr)-1] == ".0" {
			sizeStr = sizeStr[0:len(sizeStr)-3] + suffix
		}

		currentListing.size = sizeStr

	} else {
		currentListing.size = fmt.Sprintf("%d", fip.info.Size())
	}

	// epoch_nano
	currentListing.epochNano = fip.info.ModTime().UnixNano()

	// month
	currentListing.month = fip.info.ModTime().Month().String()[0:3]

	// day
	currentListing.day = fmt.Sprintf("%02d", fip.info.ModTime().Day())

	// time
	// if older than six months, print the year
	// otherwise, print hour:minute
	epochNow := time.Now().Unix()
	var secondsInSixMonths int64 = 182 * 24 * 60 * 60
	epochSixMonthsAgo := epochNow - secondsInSixMonths
	epochModified := fip.info.ModTime().Unix()

	var timeStr string
	if epochModified <= epochSixMonthsAgo ||
		epochModified >= (epochNow+5) {
		timeStr = fmt.Sprintf("%d", fip.info.ModTime().Year())
	} else {
		timeStr = fmt.Sprintf("%02d:%02d",
			fip.info.ModTime().Hour(),
			fip.info.ModTime().Minute())
	}

	currentListing.time = timeStr

	currentListing.name = fip.path

	// character?
	if fip.info.Mode()&os.ModeCharDevice == os.ModeCharDevice {
		currentListing.isCharacter = true
	} else if fip.info.Mode()&os.ModeDevice == os.ModeDevice { // block?
		currentListing.isBlock = true
	} else if fip.info.Mode()&os.ModeNamedPipe == os.ModeNamedPipe { // pipe?
		currentListing.isPipe = true
	} else if fip.info.Mode()&os.ModeSocket == os.ModeSocket { // socket?
		currentListing.isSocket = true
	}

	return currentListing, nil
}

// Given a slice of listings, return a new slice of listings with the
// directories at the front of the slice, followed by the other listings.
func sortListingsDirsFirst(listings []Listing) []Listing {

	listingsSorted := make([]Listing, 0)

	for _, l := range listings {
		if l.permissions[0] == 'd' {
			listingsSorted = append(listingsSorted, l)
		}
	}
	for _, l := range listings {
		if l.permissions[0] != 'd' {
			listingsSorted = append(listingsSorted, l)
		}
	}

	return listingsSorted
}

// Comparison function used for sorting Listings by name.
func compareName(a, b Listing) int {
	aNameLower := strings.ToLower(a.name)
	bNameLower := strings.ToLower(b.name)

	var smallerLen int
	if len(a.name) < len(b.name) {
		smallerLen = len(a.name)
	} else {
		smallerLen = len(b.name)
	}

	for i := 0; i < smallerLen; i++ {
		if aNameLower[i] < bNameLower[i] {
			return -1
		} else if aNameLower[i] > bNameLower[i] {
			return 1
		}
	}

	if len(a.name) < len(b.name) {
		return -1
	} else if len(b.name) < len(a.name) {
		return 1
	} else {
		return 0
	}
}

// Comparison function used for sorting Listings by modification time, from most
// recent to oldest.
func compareTime(a, b Listing) int {
	if a.epochNano >= b.epochNano {
		return -1
	}

	return 1
}

// Comparison function used for sorting Listings by size, from largest to
// smallest.
func compareSize(a, b Listing) int {
	a_size, _ := strconv.Atoi(a.size)
	b_size, _ := strconv.Atoi(b.size)

	if a_size >= b_size {
		return -1
	}

	return 1
}

// Sort the given listings, taking into account the current program options.
func sortListings(listings []Listing) {
	comparisonFunction := compareName
	if options.sortTime {
		comparisonFunction = compareTime
	} else if options.sortSize {
		comparisonFunction = compareSize
	}

	for {
		done := true
		for i := 0; i < len(listings)-1; i++ {
			a := listings[i]
			b := listings[i+1]

			if comparisonFunction(a, b) > -1 {
				tmp := a
				listings[i] = listings[i+1]
				listings[i+1] = tmp
				done = false
			}
		}
		if done {
			break
		}
	}

	if options.sortReverse {
		middleIndex := len(listings) / 2
		if len(listings)%2 == 0 {
			middleIndex--
		}

		for i := 0; i <= middleIndex; i++ {
			frontIndex := i
			rearIndex := len(listings) - 1 - i

			if frontIndex == rearIndex {
				break
			}

			tmp := listings[frontIndex]
			listings[frontIndex] = listings[rearIndex]
			listings[rearIndex] = tmp
		}
	}
}

// Create a set of Listings, comprised of the files and directories currently in
// the given directory.
func listFilesInDir(dir Listing) ([]Listing, error) {
	l := make([]Listing, 0)

	if options.all {
		//info_dot, err := os.Stat(dir.path)
		infoDot, err := os.Stat(dir.name)
		if err != nil {
			return l, err
		}

		listingDot, err := createListing(dir.name,
			FileInfoPath{".", infoDot})
		if err != nil {
			return l, err
		}

		infoDotdot, err := os.Stat(dir.name + "/..")
		if err != nil {
			return l, err
		}

		listingDotdot, err := createListing(dir.name,
			FileInfoPath{"..", infoDotdot})
		if err != nil {
			return l, err
		}

		l = append(l, listingDot)
		l = append(l, listingDotdot)
	}

	filesInDir, err := ioutil.ReadDir(dir.name)
	if err != nil {
		return l, err
	}

	for _, f := range filesInDir {
		// if this is a .dotfile and '-a' is not specified, skip it
		if []rune(f.Name())[0] == rune('.') && !options.all {
			continue
		}

		_l, err := createListing(dir.name,
			FileInfoPath{f.Name(), f})
		if err != nil {
			return l, err
		}
		l = append(l, _l)
	}

	sortListings(l)

	return l, nil
}

// Given a set of Listings, print them to the output buffer, taking into account
// the current program arguments and terminal width as necessary.
func writeListingsToBuffer(output_buffer *bytes.Buffer,
	listings []Listing,
	terminalWidth int) {

	if len(listings) == 0 {
		return
	}

	if options.long {
		var (
			widthPermissions  = 0
			widthNumHardLinks = 0
			widthOwner        = 0
			widthGroup        = 0
			widthSize         = 0
			widthTime         = 0
		)
		// check max widths for each field
		for _, l := range listings {
			if len(l.permissions) > widthPermissions {
				widthPermissions = len(l.permissions)
			}
			if len(l.numHardLinks) > widthNumHardLinks {
				widthNumHardLinks = len(l.numHardLinks)
			}
			if len(l.owner) > widthOwner {
				widthOwner = len(l.owner)
			}
			if len(l.group) > widthGroup {
				widthGroup = len(l.group)
			}
			if len(l.size) > widthSize {
				widthSize = len(l.size)
			}
			if len(l.time) > widthTime {
				widthTime = len(l.time)
			}
		}

		// now print the listings
		for _, l := range listings {
			// permissions
			output_buffer.WriteString(l.permissions)
			for i := 0; i < widthPermissions-len(l.permissions); i++ {
				output_buffer.WriteString(" ")
			}
			output_buffer.WriteString(" ")

			// number of hard links (right justified)
			for i := 0; i < widthNumHardLinks-len(l.numHardLinks); i++ {
				output_buffer.WriteString(" ")
			}
			for i := 0; i < 2-widthNumHardLinks; i++ {
				output_buffer.WriteString(" ")
			}
			output_buffer.WriteString(l.numHardLinks)
			output_buffer.WriteString(" ")

			// owner
			output_buffer.WriteString(l.owner)
			for i := 0; i < widthOwner-len(l.owner); i++ {
				output_buffer.WriteString(" ")
			}
			output_buffer.WriteString(" ")

			// group
			output_buffer.WriteString(l.group)
			for i := 0; i < widthGroup-len(l.group); i++ {
				output_buffer.WriteString(" ")
			}
			output_buffer.WriteString(" ")

			// size
			for i := 0; i < widthSize-len(l.size); i++ {
				output_buffer.WriteString(" ")
			}
			output_buffer.WriteString(l.size)
			output_buffer.WriteString(" ")

			// month
			output_buffer.WriteString(l.month)
			output_buffer.WriteString(" ")

			// day
			output_buffer.WriteString(l.day)
			output_buffer.WriteString(" ")

			// time
			for i := 0; i < widthTime-len(l.time); i++ {
				output_buffer.WriteString(" ")
			}
			output_buffer.WriteString(l.time)
			output_buffer.WriteString(" ")

			// name
			writeListingName(output_buffer, l)
			output_buffer.WriteString("\n")
		}
		if output_buffer.Len() > 0 {
			output_buffer.Truncate(output_buffer.Len() - 1)
		}
	} else if options.one {
		separator := "\n"

		for _, l := range listings {
			writeListingName(output_buffer, l)
			output_buffer.WriteString(separator)
		}
		if output_buffer.Len() > 0 {
			output_buffer.Truncate(output_buffer.Len() - 1)
		}
	} else {
		separator := "  "

		// calculate the number of rows needed for column output
		numRows := 1
		var colWidths []int
		for {
			numColsFloat := float64(len(listings)) / float64(numRows)
			numColsFloat = math.Ceil(numColsFloat)
			numCols := int(numColsFloat)

			colWidths = make([]int, numCols)
			for i := range colWidths {
				colWidths[i] = 0
			}

			colListings := make([]int, numCols)
			for i := 0; i < len(colListings); i++ {
				colListings[i] = 0
			}

			// calculate necessary column widths
			// also calculate the number of listings per column
			for i := 0; i < len(listings); i++ {
				col := i / numRows
				if colWidths[col] < len(listings[i].name) {
					colWidths[col] = len(listings[i].name)
				}
				colListings[col]++
			}

			// calculate the maximum width of each row
			maxRowLength := 0
			for i := 0; i < numCols; i++ {
				maxRowLength += colWidths[i]
			}
			maxRowLength += len(separator) * (numCols - 1)

			if maxRowLength > terminalWidth && numRows >= len(listings) {
				break
			} else if maxRowLength > terminalWidth {
				numRows++
			} else {
				listingsInFirstCol := colListings[0]
				listingsInLastCol := colListings[len(colListings)-1]

				// prevent short last (right-hand) columns
				if listingsInLastCol <= listingsInFirstCol/2 &&
					listingsInFirstCol-listingsInLastCol >= 5 {
					numRows++
				} else {
					break
				}
			}
		}

		for r := 0; r < numRows; r++ {
			for i, l := range listings {
				if i%numRows == r {
					writeListingName(output_buffer, l)
					for s := 0; s < colWidths[i/numRows]-len(l.name); s++ {
						output_buffer.WriteString(" ")
					}
					output_buffer.WriteString(separator)
				}
			}
			if len(listings) > 0 {
				output_buffer.Truncate(output_buffer.Len() - len(separator))
			}
			output_buffer.WriteString("\n")
		}
		output_buffer.Truncate(output_buffer.Len() - 1)
	}
}

// Parse the program arguments and write the appropriate listings to the output
// buffer.
func ls(outputBuffer *bytes.Buffer, args []string, width int) error {
	argsOptions := make([]string, 0)
	argsFiles := make([]string, 0)
	listDirs := make([]Listing, 0)
	listFiles := make([]Listing, 0)

	//
	// read in all the information from /etc/groups
	//
	groupMap = make(map[int]string)

	groupFile, err := os.Open("/etc/group")
	if err != nil {
		return fmt.Errorf("could not open /etc/group for reading\n")
	}

	reader := bufio.NewReader(groupFile)
	scanner := bufio.NewScanner(reader)

	for scanner.Scan() {
		line := scanner.Text()
		line = strings.Trim(line, " \t")

		if line[0] == '#' || line == "" {
			continue
		}

		lineSplit := strings.Split(line, ":")

		gid, err := strconv.ParseInt(lineSplit[2], 10, 0)
		if err != nil {
			return err
		}
		groupName := lineSplit[0]
		groupMap[int(gid)] = groupName
	}

	//
	// read in all information from /etc/passwd for user lookup
	//
	userMap = make(map[int]string)

	userFile, err := os.Open("/etc/passwd")
	if err != nil {
		return fmt.Errorf("could not open /etc/passwd for reading\n")
	}

	reader = bufio.NewReader(userFile)
	scanner = bufio.NewScanner(reader)

	for scanner.Scan() {
		line := scanner.Text()
		line = strings.Trim(line, " \t")

		if line[0] == '#' || line == "" {
			continue
		}

		lineSplit := strings.Split(line, ":")

		uid, err := strconv.ParseInt(lineSplit[2], 10, 0)
		if err != nil {
			return err
		}
		userName := lineSplit[0]
		userMap[int(uid)] = userName
	}

	//
	// parse arguments
	//
	for _, a := range args {
		aRune := []rune(a)
		if aRune[0] == '-' {
			// add to the options list
			argsOptions = append(argsOptions, a)
		} else {
			// add to the files/directories list
			argsFiles = append(argsFiles, a)
		}
	}

	//
	// parse options
	//
	options = Options{}
	options.color = true // use color by default
	for _, o := range argsOptions {

		// is it a short option '-' or a long option '--'?
		if strings.Contains(o, "--") {
			if strings.Contains(o, "--dirs-first") {
				options.dirsFirst = true
			}
			if strings.Contains(o, "--help") {
				options.help = true
			}
			if strings.Contains(o, "--nocolor") {
				options.color = false
			}
		} else {
			if strings.Contains(o, "1") {
				options.one = true
			}
			if strings.Contains(o, "a") {
				options.all = true
			}
			if strings.Contains(o, "d") {
				options.dir = true
			}
			if strings.Contains(o, "h") {
				options.human = true
			}
			if strings.Contains(o, "l") {
				options.long = true
			}
			if strings.Contains(o, "r") {
				options.sortReverse = true
			}
			if strings.Contains(o, "t") {
				options.sortTime = true
			}
			if strings.Contains(o, "S") {
				options.sortSize = true
			}
		}
	}

	if options.help {
		helpStr := "usage:  ls [OPTIONS] [FILES]\n\n" +
			"OPTIONS:\n" +
			"    --dirs-first  list directories first\n" +
			"    --help        display usage information\n" +
			"    --nocolor     remove color formatting\n" +
			"    -1            one entry per line\n" +
			"    -a            include entries starting with '.'\n" +
			"    -d            list directories like files\n" +
			"    -h            list sizes with human-readable units\n" +
			"    -l            long listing\n" +
			"    -r            reverse any sorting\n" +
			"    -t            sort entries by modify time\n" +
			"    -S            sort entries by size"
		outputBuffer.WriteString(helpStr)
		return nil
	}

	//
	// determine color output
	//

	if options.color {
		colorMap = make(map[string]string)
		colorMap["end"] = "\x1b[0m"

		LsColors := os.Getenv("LS_COLORS")
		LSCOLORS := os.Getenv("LSCOLORS")

		if LSCOLORS != "" {
			parseLscolors(LSCOLORS)
		} else if LsColors != "" {
			// parse LS_COLORS
			LsColorsSplit := strings.Split(LsColors, ":")
			for _, i := range LsColorsSplit {
				if i == "" {
					continue
				}

				iSplit := strings.Split(i, "=")
				colorCode := fmt.Sprintf("\x1b[%sm", iSplit[1])

				if iSplit[0] == "rs" {
					colorMap["end"] = colorCode
				} else if iSplit[0] == "di" {
					colorMap["directory"] = colorCode
				} else if iSplit[0] == "ln" {
					colorMap["symlink"] = colorCode
				} else if iSplit[0] == "mh" {
					colorMap["multi_hardlink"] = colorCode
				} else if iSplit[0] == "pi" {
					colorMap["pipe"] = colorCode
				} else if iSplit[0] == "so" {
					colorMap["socket"] = colorCode
				} else if iSplit[0] == "bd" {
					colorMap["block"] = colorCode
				} else if iSplit[0] == "cd" {
					colorMap["character"] = colorCode
				} else if iSplit[0] == "or" {
					colorMap["link_orphan"] = colorCode
				} else if iSplit[0] == "mi" {
					colorMap["link_orphan_target"] = colorCode
				} else if iSplit[0] == "su" {
					colorMap["executable_suid"] = colorCode
				} else if iSplit[0] == "sg" {
					colorMap["executable_sgid"] = colorCode
				} else if iSplit[0] == "tw" {
					colorMap["directory_o+w_sticky"] = colorCode
				} else if iSplit[0] == "ow" {
					colorMap["directory_o+w"] = colorCode
				} else if iSplit[0] == "st" {
					colorMap["directory_sticky"] = colorCode
				} else if iSplit[0] == "ex" {
					colorMap["executable"] = colorCode
				} else {
					colorMap[iSplit[0]] = colorCode
				}

				// ca - CAPABILITY? -- not supported!
				// do - DOOR -- not supported!
			}
		} else {
			// use the default LSCOLORS
			parseLscolors("exfxcxdxbxegedabagacad")
		}
	}

	// if no files are specified, list the current directory
	if len(argsFiles) == 0 {
		thisDir, _ := os.Lstat(".")
		//this_dir, _ := os.Stat(".")

		thisDirListing, err := createListing("",
			FileInfoPath{".", thisDir})
		if err != nil {
			return err
		}

		// for option_dir (-d), treat the '.' directory like a regular file
		if options.dir {
			listFiles = append(listFiles, thisDirListing)
		} else { // else, treat '.' like a directory
			listDirs = append(listDirs, thisDirListing)
		}
	}

	//
	// separate the files from the directories
	//
	for _, f := range argsFiles {
		//info, err := os.Stat(f)
		info, err := os.Lstat(f)

		if err != nil && os.IsNotExist(err) {
			return fmt.Errorf("cannot access %s: no such file or directory", f)
		} else if err != nil && os.IsPermission(err) {
			return fmt.Errorf("open %s: permission denied", f)
		} else if err != nil {
			return err
		}

		fListing, err := createListing("",
			FileInfoPath{f, info})
		if err != nil {
			return err
		}

		// for option_dir (-d), treat directories like regular files
		if options.dir {
			listFiles = append(listFiles, fListing)
		} else { // else, separate the files and directories
			if info.IsDir() {
				listDirs = append(listDirs, fListing)
			} else {
				listFiles = append(listFiles, fListing)
			}
		}
	}

	numFiles := len(listFiles)
	numDirs := len(listDirs)

	// sort the lists if necessary
	sortListings(listFiles)
	sortListings(listDirs)

	//
	// list the files first (unless --dirs-first)
	//
	if numFiles > 0 && !options.dirsFirst {
		writeListingsToBuffer(outputBuffer,
			listFiles,
			width)
	}

	//
	// then list the directories
	//
	if (numFiles > 0 && numDirs > 0) || (numDirs > 1) {
		if numFiles > 0 && !options.dirsFirst {
			outputBuffer.WriteString("\n\n")
		}

		for _, d := range listDirs {
			writeListingName(outputBuffer, d)
			outputBuffer.WriteString(":\n")

			listings, err := listFilesInDir(d)
			if err != nil {
				return err
			}

			if options.dirsFirst {
				listings = sortListingsDirsFirst(listings)
			}

			if len(listings) > 0 {
				writeListingsToBuffer(outputBuffer,
					listings,
					width)
				outputBuffer.WriteString("\n\n")
			} else {
				outputBuffer.WriteString("\n")
			}
		}

		outputBuffer.Truncate(outputBuffer.Len() - 2)
	} else if numDirs == 1 {
		for _, d := range listDirs {

			listings, err := listFilesInDir(d)
			if err != nil {
				return err
			}

			if options.dirsFirst {
				listings = sortListingsDirsFirst(listings)
			}

			writeListingsToBuffer(outputBuffer,
				listings,
				width)
		}
	}

	//
	// list the files now if --dirs-first
	//
	if numFiles > 0 && options.dirsFirst {
		if numDirs > 0 {
			outputBuffer.WriteString("\n\n")
		}
		writeListingsToBuffer(outputBuffer,
			listFiles,
			width)
	}

	return nil
}

// Main function
func main() {
	// capture the current terminal dimensions
	terminalWidth, _, err := terminal.GetSize(int(os.Stdout.Fd()))
	if err != nil {
		fmt.Printf("error getting terminal dimensions\n")
		fmt.Printf("%v\n", err)
		os.Exit(1)
	}

	var argumentList []string

	// If stdin is not a terminal, that indicates that arguments are being sent
	// via a pipe (e.g.  echo "something" | ls).  Need to include these with the
	// other program arguments.
	if !terminal.IsTerminal(int(os.Stdin.Fd())) {
		stdinBytes, err := ioutil.ReadAll(os.Stdin)
		if err != nil {
			fmt.Printf("error reading from stdin\n")
			os.Exit(1)
		}

		stdinStr := string(stdinBytes)

		// remove leading/trailing whitespace
		stdinStr = strings.TrimSpace(stdinStr)

		// remove tab characters
		stdinStr = strings.Replace(stdinStr, "\t", "", -1)

		// remove internal excess spaces
		var removedSpace bool
		for {
			removedSpace = false
			if strings.Contains(stdinStr, "  ") {
				stdinStr = strings.Replace(stdinStr, "  ", " ", -1)
				removedSpace = true
			}
			if !removedSpace {
				break
			}
		}

		stdinStrSlice := strings.Split(stdinStr, " ")

		// create a new argument list with the regular arguments followed by the
		// stdin arguments
		argumentList = append(os.Args, stdinStrSlice...)
	} else {
		argumentList = os.Args
	}

	var outputBuffer bytes.Buffer

	err = ls(&outputBuffer, argumentList[1:], terminalWidth)
	if err != nil {
		fmt.Printf("ls: %v\n", err)
		os.Exit(1)
	}

	if outputBuffer.String() != "" {
		fmt.Printf("%s\n", outputBuffer.String())
	}
}

// vim: tabstop=4 softtabstop=4 shiftwidth=4 noexpandtab tw=80