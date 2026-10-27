// Command gen writes the typed query API in internal/query from the models.
//
// Run from the module root:
//
//	go run ./_gen
//
// The leading underscore is load-bearing. Go ignores directories whose name
// starts with "_", so this is invisible to go build ./..., the linter, the test
// runner and the coverage floor — which is what a build-time tool should be,
// and which the "no cmd/ layer" rule in packages/go/README.md otherwise leaves
// nowhere to put.
//
// WHY THIS EXISTS. GORM's own query API takes column names as strings, so a
// renamed field compiles and fails at runtime against whatever the database
// happens to have. That is not a hypothetical here: a wrong route left
// subtitle_files empty for weeks, and nothing in the build had an opinion
// about it. Generated accessors move that class of mistake to compile time.
//
// The input is store.Models, the same list AutoMigrate walks. One source of
// truth: a model that is not in it has no table and now also has no query API,
// rather than having one that addresses a table that does not exist.
package main

import (
	"log"

	"gorm.io/gen"

	"homelab/managarr/internal/model"
	"homelab/managarr/internal/store"
)

func main() {
	g := gen.NewGenerator(gen.Config{
		OutPath: "internal/query",
		// WithQueryInterface emits interfaces alongside the structs, which is
		// what lets a test substitute a query without a database.
		Mode: gen.WithDefaultQuery | gen.WithQueryInterface,
		// Match the models: every optional column is already a pointer there,
		// and generating non-pointer fields would describe a different schema
		// than the one AutoMigrate creates.
		FieldNullable: true,
	})

	g.ApplyBasic(store.Models...)

	// The statements the field API cannot express, generated onto the model
	// they operate on. See querier.go.
	g.ApplyInterface(func(LibraryFileQuerier) {}, model.LibraryFile{})
	g.ApplyInterface(func(SidecarCursorQuerier) {}, model.SidecarCursor{})
	g.ApplyInterface(func(SearchQuerier) {}, model.Search{})
	g.ApplyInterface(func(GrabQuerier) {}, model.Grab{})
	g.ApplyInterface(func(ReleaseQuerier) {}, model.Release{})
	g.ApplyInterface(func(TorrentQuerier) {}, model.Torrent{})
	g.ApplyInterface(func(RipQuerier) {}, model.Rip{})
	g.ApplyInterface(func(SoulseekUploadQuerier) {}, model.SoulseekUpload{})

	g.Execute()

	log.Println("generated internal/query from", len(store.Models), "models")
}
