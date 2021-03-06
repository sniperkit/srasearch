package incrementdump

import (
	"github.com/indraniel/srasearch/ncbiparser"
	"github.com/indraniel/srasearch/sra"
	"github.com/indraniel/srasearch/utils"

	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"strings"
)

func Main(metadata, uploads, dumpfile, output string) {
	log.Println("Collecting Accession Stats from:", metadata)
	accessionDB, accession_order := ncbiparser.CollectAccessionStats(metadata)

	log.Println("Collecting Uploads Stats from:", uploads)
	uploadsDB := ncbiparser.CollectUploadStats(uploads)

	log.Println("Building Data Structure From Prior Dump:", dumpfile)
	dumpDB := collectDumpStats(dumpfile)

	log.Println("Parsing Incremental XMLs in metadata/tar file:", metadata)
	incrementalDB := ncbiparser.ProcessTarXMLs(metadata, accessionDB, uploadsDB)

	tmpdir, tmpfile := utils.MakeTmpFile()
	defer os.Remove(tmpfile)
	defer os.Remove(tmpdir)

	log.Println("Constructing intermediate dump file in tmp")
	log.Println("Merging Data Structures into Tmp Dump File:", tmpfile)
	merge(accession_order, accessionDB, dumpDB, incrementalDB, uploadsDB, tmpfile)

	log.Println("Compressing and finalizing Dump File to:", output)
	err := utils.CompressFile(tmpfile, output)
	if err != nil {
		log.Print("Trouble making gzip file:", err)
		return
	}
	log.Println("All Done!")
}

func collectDumpStats(dumpfile string) *map[string]*sra.SraItem {
	gzreader := utils.OpenGZFile(dumpfile)
	defer gzreader.Close()

	db := make(map[string]*sra.SraItem)

	count := 1
	reader := bufio.NewReader(gzreader.Gzf)
	line, err := reader.ReadString('\n')

	for err == nil {
		elems := strings.SplitN(line, ",", 2)
		accession, jsonData := elems[0], elems[1]
		raw := strings.TrimRight(jsonData, "\n")
		si := new(sra.SraItem)
		if e := json.Unmarshal([]byte(raw), si); e != nil {
			log.Fatalf(
				"Trouble json parsing accession record: %s : %s\n",
				raw, e,
			)
		}
		db[accession] = si
		line, err = reader.ReadString('\n')
		count++
	}

	if err != io.EOF {
		log.Fatalln(
			"[err] reading line ", count, "in", gzreader.File.Name(), ":", err,
		)
	}

	return &db
}

func merge(
	accessions []string,
	accessionDB *map[string]*sra.AccessionRecord,
	dumpDB *map[string]*sra.SraItem,
	incrementalDB *map[string]*sra.SraItem,
	uploadsDB *map[string][]string,
	outFile string,
) {

	outPtr, err := os.Create(outFile)
	if err != nil {
		log.Fatal("Trouble opening %s for writing : %s\n", outFile, err)
	}
	defer outPtr.Close()

	for i, accession := range accessions {
		// the "hot" stuff should be in the incremental file
		if sraItem, ok := (*incrementalDB)[accession]; ok {
			sraItem.Record(outPtr)
			continue
		}

		// the "usual" stuff should be in the prior dump
		if sraItem, ok := (*dumpDB)[accession]; ok {
			sraItem.Record(outPtr)
			continue
		}

		// problematic cases -- specially handle and note
		if accessionRecord, ok := (*accessionDB)[accession]; ok {
			fmt.Printf(
				"--> [%d] Got a NCBI record with no XML : %s (%s)\n",
				i, accession, accessionRecord.Status,
			)
			sraItem := new(sra.SraItem)
			sraItem.Id = accession
			sraItem.AddAttrFromAccessionRecords(accessionDB)
			sraItem.AddAttrFromUploadRecords(uploadsDB)
			sraItem.Record(outPtr)
			continue
		}

		// this shouldn't be happening...but you never know...
		log.Fatalln("[err] Don't know how to merge accession: ", accession, "!")
	}
}
