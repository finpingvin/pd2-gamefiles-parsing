package main

import (
	"encoding/binary"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"strconv"
	"strings"
)

type monster struct {
	DisplayName     string `json:"displayName"`
	PhysRes         int    `json:"physRes"`
	MagicRes        int    `json:"magicRes"`
	FireRes         int    `json:"fireRes"`
	LightningRes    int    `json:"lightningRes"`
	ColdRes         int    `json:"coldRes"`
	PoisonRes       int    `json:"poisonRes"`
	MinHpClosedBnet int    `json:"minHpClosedBnet"`
	MaxHpClosedBnet int    `json:"maxHpClosedBnet"`
	MinHpOpenBnet   int    `json:"minHpOpenBnet"`
	MaxHpOpenBnet   int    `json:"maxHpOpenBnet"`
}

type mapLevel struct {
	DisplayName string     `json:"displayName"`
	Tier        int        `json:"tier"`
	Monsters    *[]monster `json:"monsters"`
}

func readBytes(f *os.File, delim byte) ([]byte, error) {
	buffer := make([]byte, 1)
	content := []byte{}
	for {
		if _, err := f.Read(buffer); err != nil {
			return nil, err
		}
		if buffer[0] == delim {
			return content, nil
		}
		content = append(content, buffer[0])
	}
}

func readTblElements(f *os.File, numberOfElements uint16) []uint16 {
	elements := make([]uint16, numberOfElements)
	// Reads in 2 bytes for each element.  This is the offset into the hash array for the element with this number.
	for i := 0; i < int(numberOfElements); i++ {
		elementRaw := make([]byte, 2)
		if _, err := f.Read(elementRaw); err != nil {
			log.Fatal(err)
		}
		elements[i] = binary.LittleEndian.Uint16(elementRaw)
	}
	return elements
}

func readTbl(tblFile string) map[string]string {
	f, err := os.Open(tblFile)
	if err != nil {
		log.Fatal(err)
	}

	// Skip the CRC header
	if _, err := f.Seek(2, io.SeekCurrent); err != nil {
		log.Fatal(err)
	}
	// This is usNumElements, the total number of elements (key/value string pairs) in the file.
	numberOfElementsRaw := make([]byte, 2)
	if _, err := f.Read(numberOfElementsRaw); err != nil {
		log.Fatal(err)
	}
	numberOfElements := binary.LittleEndian.Uint16(numberOfElementsRaw)
	// hashtablesize
	if _, err := f.Seek(4, io.SeekCurrent); err != nil {
		log.Fatal(err)
	}
	// dont know what this is used for
	if _, err := f.Seek(1, io.SeekCurrent); err != nil {
		log.Fatal(err)
	}
	// This is dwIndexStart, the offset of the first byte of the actual strings.  This offset is from the start of the file, as are the other offsets mentioned herein.  We don't really need it when reading.
	if _, err := f.Seek(4, io.SeekCurrent); err != nil {
		log.Fatal(err)
	}
	// When the number of times you have missed a match with a hash key equals this value, you give up because it is not there.  We don't care what this value was in the original.
	if _, err := f.Seek(4, io.SeekCurrent); err != nil {
		log.Fatal(err)
	}
	// This is dwIndexEnd, the offset just after the last byte of the actual strings.
	if _, err := f.Seek(4, io.SeekCurrent); err != nil {
		log.Fatal(err)
	}
	// Read the positions of all elements
	elements := readTblElements(f, numberOfElements)
	// Get current file offset so we can later calculate
	// the key and value offsets
	nodeStart, err := f.Seek(0, io.SeekCurrent)
	if err != nil {
		log.Fatal(err)
	}

	// Will contain all key/values from the tbl file
	tbl := make(map[string]string)
	// Iterate elements to read their keys and values from the element offset.
	for i := 0; i < len(elements); i++ {
		// Set the offset that we are reading from in the file to be (Start of Hash Table + (17 * the ith element in array elements)), meaning the start of the hash table entry that was indicated by entry #i in the previous table.
		if _, err := f.Seek(nodeStart+(int64(elements[i])*17), io.SeekStart); err != nil {
			log.Fatal(err)
		}
		// This is is bUsed, which is set to 1 if this entry is used.
		if _, err := f.Seek(1, io.SeekCurrent); err != nil {
			log.Fatal(err)
		}
		// This is the index number.  Basically, this should always be equal to the value of $i as we read it.
		if _, err := f.Seek(2, io.SeekCurrent); err != nil {
			log.Fatal(err)
		}
		// This is the number you get from sending this entry's key string through the hashing algorithim.  We don't care about it right now.
		if _, err := f.Seek(4, io.SeekCurrent); err != nil {
			log.Fatal(err)
		}
		// This is dwKeyOffset, the offset of the key string.  The key is the same in every language.
		keyOffsetRaw := make([]byte, 4)
		if _, err := f.Read(keyOffsetRaw); err != nil {
			log.Fatal(err)
		}
		keyOffset := binary.LittleEndian.Uint32(keyOffsetRaw)
		// This is dwStringOffset, the offset to the value string.  The value is translated into the appropriate language.
		stringOffsetRaw := make([]byte, 4)
		if _, err := f.Read(stringOffsetRaw); err != nil {
			log.Fatal(err)
		}
		stringOffset := binary.LittleEndian.Uint32(stringOffsetRaw)
		// This is the length of the value string.
		if _, err := f.Seek(2, io.SeekCurrent); err != nil {
			log.Fatal(err)
		}
		// Go to the key's offset now.
		if _, err := f.Seek(int64(keyOffset), io.SeekStart); err != nil {
			log.Fatal(err)
		}
		// read into local variable $key everything up to and including the next null byte, discarding the null.
		keyRaw, err := readBytes(f, byte(0))
		if err != nil {
			log.Fatal(err)
		}
		key := string(keyRaw)
		// Go to the value string's offset
		if _, err := f.Seek(int64(stringOffset), io.SeekStart); err != nil {
			log.Fatal(err)
		}
		// read into local variable $string everything up to and including the next null byte, discarding the null.
		valueRaw, err := readBytes(f, byte(0))
		if err != nil {
			log.Fatal(err)
		}
		value := string(valueRaw)
		tbl[key] = value
	}

	return tbl
}

func readDataFile(filename string) []map[string]string {
	f, err := os.Open(filename)
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()

	r := csv.NewReader(f)
	r.Comma = '\t'

	header, err := r.Read()
	if err != nil {
		log.Fatal(err)
	}

	items := make([]map[string]string, len(header))

	for {
		record, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			log.Fatal(err)
		}

		item := make(map[string]string)

		for i, key := range header {
			item[strings.TrimSpace(key)] = strings.TrimSpace(record[i])
		}

		items = append(items, item)
	}

	return items
}

func groupMonsterLevelsById(monsterLevels []map[string]string) map[string]map[string]string {
	monsterLevelsById := make(map[string]map[string]string)
	for _, monsterLevel := range monsterLevels {
		monsterLevelsById[monsterLevel["Level"]] = map[string]string{
			"hpClosedBnet": monsterLevel["HP(H)"],
			"hpOpenBnet":   monsterLevel["L-HP(H)"],
		}
	}

	return monsterLevelsById
}

func getMapsFromMisc(miscItems []map[string]string) map[string]mapLevel {
	mapLevels := make(map[string]mapLevel)
	mapTiers := map[string]int{
		"t1m": 1,
		"t2m": 2,
		"t3m": 3,
		"t4m": 4,
		"t5m": 5,
	}

	for _, item := range miscItems {
		t := item["type"]
		spawnable := item["spawnable"]
		// The unique maps still have spawnable="0", so special case for them
		if mapTier, ok := mapTiers[t]; ok && (spawnable == "1" || t == "t5m") {
			mapLevels[item["len"]] = mapLevel{
				DisplayName: item["*name"],
				Tier:        mapTier,
				// TODO: I don't think this is the right way to handle
				// this in Go, but it works for now
				Monsters: new([]monster),
			}
		}
	}

	return mapLevels
}

func main() {
	stringString := readTbl("./string.tbl")
	patchString := readTbl("./patchstring.tbl")
	expansionString := readTbl("./expansionstring.tbl")

	monsters := readDataFile("./MonStats.txt")
	monsterLevels := groupMonsterLevelsById(readDataFile("./MonLvl.txt"))
	misc := readDataFile("./Misc.txt")
	levels := readDataFile("./Levels.txt")
	mapLevels := getMapsFromMisc(misc)
	output := make([]mapLevel, 0)

	for _, l := range levels {
		ml, ok := mapLevels[l["Id"]]
		if !ok {
			// Not a map, skip
			continue
		}
		fmt.Println("Found map", l["Name"], ml.DisplayName)

		for _, m := range monsters {
			if m["NameStr"] == "" {
				continue
			}

			if l["mon1"] == m["Id"] ||
				l["mon2"] == m["Id"] ||
				l["mon3"] == m["Id"] ||
				l["mon4"] == m["Id"] ||
				l["mon5"] == m["Id"] ||
				l["mon6"] == m["Id"] ||
				l["mon7"] == m["Id"] ||
				l["mon8"] == m["Id"] ||
				l["mon9"] == m["Id"] ||
				l["mon10"] == m["Id"] ||
				l["mon11"] == m["Id"] ||
				l["mon12"] == m["Id"] ||
				l["mon13"] == m["Id"] ||
				l["mon14"] == m["Id"] ||
				l["mon15"] == m["Id"] {

				monsterPhysRes, err := strconv.Atoi(m["ResDm(H)"])
				if err != nil {
					monsterPhysRes = 0
				}
				monsterMagicRes, err := strconv.Atoi(m["ResMa(H)"])
				if err != nil {
					monsterMagicRes = 0
				}
				monsterFireRes, err := strconv.Atoi(m["ResFi(H)"])
				if err != nil {
					monsterFireRes = 0
				}
				monsterLightningRes, err := strconv.Atoi(m["ResLi(H)"])
				if err != nil {
					monsterLightningRes = 0
				}
				monsterColdRes, err := strconv.Atoi(m["ResCo(H)"])
				if err != nil {
					monsterColdRes = 0
				}
				monsterPoisonRes, err := strconv.Atoi(m["ResPo(H)"])
				if err != nil {
					monsterPoisonRes = 0
				}

				var monsterName string
				if str, ok := stringString[m["NameStr"]]; ok && strings.TrimSpace(str) != "" {
					monsterName = str
				} else if str, ok := patchString[m["NameStr"]]; ok && strings.TrimSpace(str) != "" {
					monsterName = str
				} else if str, ok := expansionString[m["NameStr"]]; ok && strings.TrimSpace(str) != "" {
					monsterName = str
				} else {
					fmt.Println("Could not match display name monster", m["NameStr"])
					monsterName = m["NameStr"]
				}

				monsterLevel := monsterLevels[l["MonLvl3"]]
				hpMultiplierClosedBnet, _ := strconv.Atoi(monsterLevel["hpClosedBnet"])
				hpMultiplierOpenBnet, _ := strconv.Atoi(monsterLevel["hpOpenBnet"])
				baseMinHp, _ := strconv.Atoi(m["minHP"])
				baseMaxHp, _ := strconv.Atoi(m["maxHP"])
				minHpClosedBnet := (hpMultiplierClosedBnet * baseMinHp) / 100
				maxHpClosedBnet := (hpMultiplierClosedBnet * baseMaxHp) / 100
				minHpOpenBnet := (hpMultiplierOpenBnet * baseMinHp) / 100
				maxHpOpenBnet := (hpMultiplierOpenBnet * baseMaxHp) / 100

				*ml.Monsters = append(*ml.Monsters, monster{
					DisplayName:     monsterName,
					PhysRes:         monsterPhysRes,
					MagicRes:        monsterMagicRes,
					FireRes:         monsterFireRes,
					LightningRes:    monsterLightningRes,
					ColdRes:         monsterColdRes,
					PoisonRes:       monsterPoisonRes,
					MinHpClosedBnet: minHpClosedBnet,
					MaxHpClosedBnet: maxHpClosedBnet,
					MinHpOpenBnet:   minHpOpenBnet,
					MaxHpOpenBnet:   maxHpOpenBnet,
				})
			}
		}

		output = append(output, ml)
	}

	mapLevelsJson, err := json.Marshal(output)
	if err != nil {
		log.Fatal(err)
	}

	err = os.WriteFile("./maps.json", mapLevelsJson, 0644)
	if err != nil {
		log.Fatal(err)
	}
}
