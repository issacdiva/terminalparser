package terminalparser

import (
	"bytes"
	"log"
	"strconv"
	"unicode/utf8"

	"github.com/mattn/go-runewidth"
)

type Screen struct {
	Rows []*Row

	cursor *Cursor

	pasteMode bool // Set bracketed paste mode, xterm. ?2004h   reset ?2004l

	title string
}

func (s *Screen) Parse(data []byte) []string {
	s.cursor.Y = 1
	s.Rows = append(s.Rows, &Row{
		dataRune: make([]rune, 0, 1024),
	})
	rest := data
	for len(rest) > 0 {
		code, size := utf8.DecodeRune(rest)
		rest = rest[size:]
		switch code {
		case ESCKey:
			code, size = utf8.DecodeRune(rest)
			rest = rest[size:]
			switch code {
			case '[':
				// CSI
				rest = s.parseCSISequence(rest)
				continue
			case ']':
				// OSC
				rest = s.parseOSCSequence(rest)
				continue
			default:
				if existIndex := bytes.IndexRune([]byte(string(Intermediate)), code); existIndex >= 0 {
					// ESC
					rest = s.parseIntermediate(code, rest)
					continue
				}
				if existIndex := bytes.IndexRune([]byte(string(Parameters)), code); existIndex >= 0 {

					switch code {
					case '=':
						log.Println("不支持=")
					case '>':
						log.Println("不支持>")
					default:
						log.Printf("Parameters====`%q` %x\n", code, code)
					}

					continue
				}

				if existIndex := bytes.IndexRune([]byte(string(Uppercase)), code); existIndex >= 0 {
					log.Printf("Uppercase====`%q` %x\n", code, code)
					continue
				}

				if existIndex := bytes.IndexRune([]byte(string(Lowercase)), code); existIndex >= 0 {
					log.Printf("Lowercase====`%q` %x\n", code, code)
					continue
				}
				log.Printf("未识别的解析，ESCKey `%q` %x\n", code, code)
			}
			continue
		case Delete:
			continue
		default:
			if existIndex := bytes.IndexRune([]byte(string(C0Control)), code); existIndex >= 0 {
				s.parseC0Sequence(code)
			} else {
				if len(s.Rows) == 0 && s.cursor.Y == 0 {
					s.Rows = append(s.Rows, &Row{
						dataRune: make([]rune, 0, 1024),
					})
					s.cursor.Y++
				}
				s.appendCharacter(code)
			}
			continue
		}

	}
	result := make([]string, len(s.Rows))
	for i := range s.Rows {
		result[i] = s.Rows[i].String()
	}
	return result
}

func (s *Screen) parseC0Sequence(code rune) {
	switch code {
	case 0x07:
		//bell 忽略
	case 0x08:
		// 后退1光标
		s.cursor.MoveLeft(1)
	case 0x0d:
		/*
			\r
		*/
		s.cursor.X = 0
	case 0x0a:
		/*
			\n
		*/
		s.cursor.Y++
		if s.cursor.Y > len(s.Rows) {
			s.Rows = append(s.Rows, &Row{
				dataRune: make([]rune, 0, 1024),
			})
		}
	default:
		log.Printf("未处理的字符 %q %v\n", code, code)
	}

}

func (s *Screen) parseCSISequence(p []byte) []byte {
	endIndex := bytes.IndexFunc(p, IsAlphabetic)
	params := []rune(string(p[:endIndex]))
	switch rune(p[endIndex]) {
	case 'Y':
		//	/*
		//		ESC Y Ps Ps
		//		          Move the cursor to given row and column.
		//	*/
		if len(p[endIndex+1:]) < 2 {
			return p[endIndex+1:]
		}
		if row, err := strconv.Atoi(string(p[endIndex+1])); err == nil {
			s.cursor.Y = row
		}
		if col, err := strconv.Atoi(string(p[endIndex+2])); err == nil {
			s.cursor.X = col
		}
		return p[endIndex+3:]

	}

	funcName, ok := CSIFuncMap[rune(p[endIndex])]
	if ok {
		funcName(s, params)
	} else {
		log.Printf("screen未处理的CSI %s %q\n", DebugString(string(params)), p[endIndex])
	}

	return p[endIndex+1:]
}

func (s *Screen) parseIntermediate(code rune, p []byte) []byte {
	switch code {
	case '(':
		terminationIndex := bytes.IndexFunc(p, func(r rune) bool {
			if insideIndex := bytes.IndexRune([]byte(string(Alphabetic)), r); insideIndex < 0 {
				return false
			}
			return true
		})
		params := p[:terminationIndex+1]
		switch string(params) {
		case "B":
			/*
				ESC ( C   Designate G0 Character Set, VT100, ISO 2022.

						  C = B  ⇒  United States (USASCII), VT100.

			*/
		}
		p = p[terminationIndex+1:]
		return p
	case ')':
		terminationIndex := bytes.IndexFunc(p, func(r rune) bool {
			if insideIndex := bytes.IndexRune([]byte(string(Alphabetic)), r); insideIndex < 0 {
				return false
			}
			return true
		})
		p = p[terminationIndex+1:]
	default:
		log.Printf("未处理的 parseIntermediate %q %d\n", code, code)
	}
	return p
}

func (s *Screen) parseOSCSequence(p []byte) []byte {
	if endIndex := bytes.IndexRune(p, BEL); endIndex >= 0 {
		return p[endIndex+1:]
	}

	if endIndex := bytes.IndexRune(p, ST); endIndex >= 0 {
		return p[endIndex+1:]
	}
	log.Println("未处理的 parseOSCSequence")
	return p
}

func (s *Screen) appendCharacter(code rune) {
	currentRow := s.GetCursorRow()
	currentRow.changeCursorToX(s.cursor.X)
	currentRow.appendCharacter(code)
	width := runewidth.StringWidth(string(code))
	s.cursor.X += width
}

func (s *Screen) eraseEndToLine() {
	log.Printf("eraseEndToLine %d %d %d\n", s.cursor.X,
		s.cursor.Y, len(s.Rows))
	currentRow := s.GetCursorRow()
	currentRow.changeCursorToX(s.cursor.X)
	currentRow.eraseRight()

}

func (s *Screen) eraseRight() {
	log.Printf("eraseRight %d %d %d\n", s.cursor.X,
		s.cursor.Y, len(s.Rows))
	currentRow := s.GetCursorRow()
	currentRow.changeCursorToX(s.cursor.X)
	currentRow.eraseRight()
}

func (s *Screen) eraseLeft() {
	log.Printf("Screen %s EraseLeft， cursor(%d，%d),总Row数量 %d",
		UnsupportedMsg, s.cursor.X, s.cursor.Y, len(s.Rows))
}

func (s *Screen) eraseAbove() {
	log.Printf("eraseAbove %d %d %d", s.cursor.X,
		s.cursor.Y, len(s.Rows))
	s.Rows = s.Rows[s.cursor.Y-1:]
}

func (s *Screen) eraseBelow() {
	log.Printf("eraseBelow %d %d %d", s.cursor.X,
		s.cursor.Y, len(s.Rows))
	s.Rows = s.Rows[:s.cursor.Y]
}

func (s *Screen) eraseAll() {
	log.Printf("eraseAll %d %d %d", s.cursor.X,
		s.cursor.Y, len(s.Rows))
	s.Rows = s.Rows[:0]
	//htop?
	s.cursor.X = 0
	s.cursor.Y = 0
}

func (s *Screen) eraseFromCursor() {
	log.Printf("eraseFromCursor %d %d %d", s.cursor.X,
		s.cursor.Y, len(s.Rows))
	if s.cursor.Y > len(s.Rows) {
		s.cursor.Y = len(s.Rows)
	}
	s.Rows = s.Rows[:s.cursor.Y]
	s.Rows[s.cursor.Y-1].changeCursorToX(s.cursor.X)
	s.Rows[s.cursor.Y-1].eraseRight()
}

func (s *Screen) deleteChars(ps int) {
	log.Printf("deleteChars %d chars \n", ps)
	currentRow := s.GetCursorRow()
	currentRow.changeCursorToX(s.cursor.X)
	currentRow.deleteChars(ps)
}

func (s *Screen) GetCursorRow() *Row {
	if s.cursor.Y == 0 {
		s.cursor.Y++
	}
	if len(s.Rows) == 0 {
		s.Rows = append(s.Rows, &Row{
			dataRune: make([]rune, 0, 1024),
		})
	}
	index := s.cursor.Y - 1
	if index >= len(s.Rows) {
		log.Printf("总行数 %d 比当前行 %d 小，存在解析错误 \n", len(s.Rows), s.cursor.Y)
		return s.Rows[len(s.Rows)-1]
	}
	return s.Rows[s.cursor.Y-1]
}

const UnsupportedMsg = "Unsupported"
