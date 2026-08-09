// Copyright 2026 asadtariq96 and contributors. Licensed under Apache-2.0. See LICENSE.
// Transcendence commands: deals (price+discount+rating+stock scoring), categories
// (facet-tree drill-down) and watch (price/stock-change tracking) for Verkkokauppa.com.

package cli

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"verkkokauppa-pp-cli/internal/client"
)

const (
	vkSearchURL = "https://search.service.verkkokauppa.com/fi/api/v1/product-search"
	vkAvailURL  = "https://product.service.verkkokauppa.com/fi/api/v1/availability"
)

func vkNewSessionID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

// ---- types mirroring the JSON:API search envelope ----

type vkSearchEnv struct {
	Data     []json.RawMessage `json:"data"`
	Included []json.RawMessage `json:"included"`
}

type vkProduct struct {
	ID         string         `json:"id"`
	Attributes vkProductAttrs `json:"attributes"`
}
type vkProductAttrs struct {
	Name     string      `json:"name"`
	Articles []vkArticle `json:"articles"`
	Price    vkPrice     `json:"price"`
	Rating   struct {
		AverageOverallRating float64 `json:"averageOverallRating"`
	} `json:"rating"`
}
type vkArticle struct {
	Name  string  `json:"name"`
	Price vkPrice `json:"price"`
}
type vkPrice struct {
	Current  float64 `json:"current"`
	Original float64 `json:"original"`
}

type vkAvail struct {
	Pid    int `json:"pid"`
	Status struct {
		Schema string `json:"schema"`
	} `json:"status"`
}

type vkFacet struct {
	Type       string `json:"type"`
	ID         string `json:"id"`
	Attributes struct {
		Title  string         `json:"title"`
		Values []vkFacetValue `json:"values"`
	} `json:"attributes"`
}
type vkFacetValue struct {
	Key      string         `json:"key"`
	Title    string         `json:"title"`
	Hits     int            `json:"hits"`
	Children []vkFacetValue `json:"children"`
}

func (p *vkProduct) price() (cur, orig float64) {
	if p.Attributes.Price.Current > 0 {
		return p.Attributes.Price.Current, p.Attributes.Price.Original
	}
	if len(p.Attributes.Articles) > 0 && p.Attributes.Articles[0].Price.Current > 0 {
		pr := p.Attributes.Articles[0].Price
		return pr.Current, pr.Original
	}
	return 0, 0
}

func vkSearchParams(q, category, sessionID string, pageSize int) map[string]string {
	p := map[string]string{
		"page[number]": "1",
		"page[size]":   strconv.Itoa(pageSize),
		"sort":         "-score",
		"sessionId":    sessionID,
		"include":      "campaigns,category,salesCategories.parent,facets",
	}
	if q != "" {
		p["filter[q]"] = q
	}
	if category != "" {
		p["filter[category]"] = category
	}
	return p
}

func vkFetchSearchEnv(ctx context.Context, c *client.Client, params map[string]string) (*vkSearchEnv, error) {
	raw, err := c.Get(ctx, vkSearchURL, params)
	if err != nil {
		return nil, err
	}
	var env vkSearchEnv
	if err := json.Unmarshal(raw, &env); err != nil {
		return nil, fmt.Errorf("parsing search response: %w", err)
	}
	return &env, nil
}

func vkProducts(env *vkSearchEnv) ([]*vkProduct, error) {
	out := make([]*vkProduct, 0, len(env.Data))
	for _, rm := range env.Data {
		var p vkProduct
		if err := json.Unmarshal(rm, &p); err != nil {
			return nil, err
		}
		out = append(out, &p)
	}
	return out, nil
}

func vkAvailability(ctx context.Context, c *client.Client, pids []string) (map[string]string, error) {
	if len(pids) == 0 {
		return map[string]string{}, nil
	}
	raw, err := c.Get(ctx, vkAvailURL, map[string]string{"pids": strings.Join(pids, ",")})
	if err != nil {
		return nil, err
	}
	var list []vkAvail
	if err := json.Unmarshal(raw, &list); err != nil {
		return nil, err
	}
	m := make(map[string]string, len(list))
	for _, a := range list {
		m[strconv.Itoa(a.Pid)] = a.Status.Schema
	}
	return m, nil
}

// ---- scoring (deal synthesis) ----

func vkScore(cur, orig, rating float64, stock string) float64 {
	disc := 0.0
	if orig > 0 {
		disc = (orig - cur) / orig * 100
		if disc < 0 {
			disc = 0
		}
		if disc > 50 {
			disc = 50
		}
	}
	s := disc * 0.4 // discount component, 0..20
	switch stock {
	case "InStock":
		s += 25
	case "LimitedAvailability":
		s += 15
	case "SoldOut", "OutOfStock", "Preorder", "NotYetReleased":
		s += 5
	default:
		s += 10
	}
	if rating < 0 {
		rating = 0
	}
	s += rating * 6 // 0..30
	switch {
	case cur > 0 && cur <= 150:
		s += 25
	case cur > 0 && cur <= 400:
		s += 20
	case cur > 0 && cur <= 1000:
		s += 12
	case cur > 0 && cur <= 2500:
		s += 6
	}
	if s > 100 {
		s = 100
	}
	return s
}

// ---- deals command ----

func newVkCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "vk",
		Short: "Verkkokauppa.com analysis commands: deals, categories, watch",
		Long: "Transcendence commands layered on the Verkkokauppa.com product API: " +
			"deal scoring (price + discount + rating + availability), category facet " +
			"drill-down, and price/stock change tracking.",
	}
	cmd.AddCommand(newVkDealsCmd(flags))
	cmd.AddCommand(newVkCategoriesCmd(flags))
	cmd.AddCommand(newVkWatchCmd(flags))
	return cmd
}

func newVkDealsCmd(flags *rootFlags) *cobra.Command {
	var pageSize int
	var minDiscount, minScore float64
	var category, priceRange string
	var asJSON bool
	cmd := &cobra.Command{
		Use:   "deals QUERY",
		Short: "Find best-value products: price + discount + rating + availability, scored 0-100",
		Long: "Searches Verkkokauppa.com for QUERY, pulls live stock, and scores every product " +
			"(discount%, availability, rating and price band -> 0-100 'worth buying' score). " +
			"Highest-scoring deals first. Use --min-discount to surface genuine markdowns.",
		Example: "  verkkokauppa-pp-cli vk deals \"näytönohjain\" --min-discount 10 --size 20\n" +
			"  verkkokauppa-pp-cli vk deals \"iphone\" --category information_technology --json",
		Args: cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			q := strings.TrimSpace(strings.Join(args, " "))
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			sid := vkSessionID(flags)
			params := vkSearchParams(q, category, sid, pageSize)
			if priceRange != "" {
				params["filter[price]"] = priceRange
			}
			env, err := vkFetchSearchEnv(cmd.Context(), c, params)
			if err != nil {
				return err
			}
			prods, err := vkProducts(env)
			if err != nil {
				return err
			}
			pids := make([]string, 0, len(prods))
			for _, p := range prods {
				if p.ID != "" {
					pids = append(pids, p.ID)
				}
			}
			avail, err := vkAvailability(cmd.Context(), c, pids)
			if err != nil {
				return err
			}
			type row struct {
				PID, Name               string
				Cur, Orig, Disc, Rating float64
				Stock                   string
				Score                   float64
			}
			rows := make([]row, 0, len(prods))
			for _, p := range prods {
				cur, orig := p.price()
				disc := 0.0
				if orig > 0 {
					disc = (orig - cur) / orig * 100
				}
				stock := avail[p.ID]
				score := vkScore(cur, orig, p.Attributes.Rating.AverageOverallRating, stock)
				if disc < minDiscount || score < minScore {
					continue
				}
				rows = append(rows, row{p.ID, p.Attributes.Name, cur, orig, disc,
					p.Attributes.Rating.AverageOverallRating, stock, score})
			}
			sort.Slice(rows, func(i, j int) bool { return rows[i].Score > rows[j].Score })
			if asJSON {
				return vkPrintJSON(cmd.OutOrStdout(), rows)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%-5s %-7s %-52s %9s %9s %6s %-20s %5s\n",
				"SCORE", "PID", "NAME", "PRICE", "WAS", "-%", "STOCK", "RATING")
			for _, r := range rows {
				fmt.Fprintf(cmd.OutOrStdout(), "%5.0f %-7s %-52s %9.2f %9.2f %5.1f%% %-20s %5.1f\n",
					r.Score, r.PID, trunc(r.Name, 52), r.Cur, r.Orig, r.Disc, r.Stock, r.Rating)
			}
			if len(rows) >= pageSize {
				fmt.Fprintf(os.Stderr, "\nShowing %d deals. Narrow with --category / --price / --min-discount.\n", len(rows))
			}
			return nil
		},
	}
	cmd.Flags().IntVar(&pageSize, "size", 24, "How many search results to score")
	cmd.Flags().Float64Var(&minDiscount, "min-discount", 0, "Only show items with at least this discount %")
	cmd.Flags().Float64Var(&minScore, "min-score", 0, "Only show items scored at least this (0-100)")
	cmd.Flags().StringVar(&category, "category", "", "Category facet key to narrow results")
	cmd.Flags().StringVar(&priceRange, "price", "", "Price range as min-max in euros")
	cmd.Flags().BoolVar(&asJSON, "json", false, "Output as JSON")
	return cmd
}

// ---- categories command ----

func newVkCategoriesCmd(flags *rootFlags) *cobra.Command {
	var query, category string
	var depth int
	var asJSON bool
	cmd := &cobra.Command{
		Use:   "categories",
		Short: "Browse the category facet tree (drill-down by --category)",
		Long: "Runs a search (optionally scoped by --query / --category) and renders the " +
			"category facet tree with hit counts. Pass --category to drill into a sub-tree.",
		Example: "  verkkokauppa-pp-cli vk categories --query \"ssd\" --depth 3\n" +
			"  verkkokauppa-pp-cli vk categories --category information_technology --json",
		RunE: func(cmd *cobra.Command, args []string) error {
			if query == "" {
				query = strings.TrimSpace(strings.Join(args, " "))
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			sid := vkSessionID(flags)
			env, err := vkFetchSearchEnv(cmd.Context(), c, vkSearchParams(query, category, sid, 1))
			if err != nil {
				return err
			}
			var tree *vkFacet
			for _, rm := range env.Included {
				var f vkFacet
				if json.Unmarshal(rm, &f) != nil || f.Type != "facet" || f.ID != "category" {
					continue
				}
				ff := f
				tree = &ff
				break
			}
			if tree == nil {
				return fmt.Errorf("no category facet returned (query=%q category=%q); try a different query", query, category)
			}
			if asJSON {
				return vkPrintJSON(cmd.OutOrStdout(), tree.Attributes.Values)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Category facets for query=%q category=%q :\n", query, category)
			maxDepth := depth
			if maxDepth == 0 {
				maxDepth = 1000
			}
			vkPrintFacetValues(cmd, tree.Attributes.Values, 0, maxDepth)
			return nil
		},
	}
	cmd.Flags().StringVar(&query, "query", "", "Search query that determines the facet tree")
	cmd.Flags().StringVar(&category, "category", "", "Category key to drill into")
	cmd.Flags().IntVar(&depth, "depth", 0, "Max tree depth (0 = all)")
	cmd.Flags().BoolVar(&asJSON, "json", false, "Output as JSON")
	return cmd
}

func vkPrintFacetValues(cmd *cobra.Command, vals []vkFacetValue, d, maxDepth int) {
	if d >= maxDepth {
		return
	}
	indent := strings.Repeat("   ", d)
	for _, v := range vals {
		fmt.Fprintf(cmd.OutOrStdout(), "%s%s (%d) %s\n", indent, v.Key, v.Hits, v.Title)
		if len(v.Children) > 0 {
			vkPrintFacetValues(cmd, v.Children, d+1, maxDepth)
		}
	}
}

// ---- watch command ----

type vkWatchesFile struct {
	Watches []vkWatch `json:"watches"`
}
type vkWatch struct {
	ID       int                   `json:"id"`
	Query    string                `json:"query"`
	Category string                `json:"category"`
	Snapshot map[string]vkSnapshot `json:"snapshot"`
}
type vkSnapshot struct {
	Name      string  `json:"name"`
	Price     float64 `json:"price"`
	Stock     string  `json:"stock"`
	PrevPrice float64 `json:"prevPrice"`
	PrevStock string  `json:"prevStock"`
}

func vkWatchesPath() string {
	if d := os.Getenv("VERKKOKAUPPA_DATA_DIR"); d != "" {
		return filepath.Join(d, "watch.json")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		home = "."
	}
	return filepath.Join(home, ".local", "share", "verkkokauppa-pp-cli", "watch.json")
}

func vkLoadWatches() (*vkWatchesFile, error) {
	w := &vkWatchesFile{}
	data, err := os.ReadFile(vkWatchesPath())
	if err != nil {
		if os.IsNotExist(err) {
			return w, nil
		}
		return nil, err
	}
	if err := json.Unmarshal(data, w); err != nil {
		return nil, fmt.Errorf("parsing watch state: %w", err)
	}
	return w, nil
}

func vkSaveWatches(w *vkWatchesFile) error {
	p := vkWatchesPath()
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(w, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(p, data, 0o600)
}

func newVkWatchCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "watch",
		Short: "Track price and stock changes for saved searches",
	}
	cmd.AddCommand(newVkWatchAddCmd(flags))
	cmd.AddCommand(newVkWatchListCmd(flags))
	cmd.AddCommand(newVkWatchRemoveCmd(flags))

	check := &cobra.Command{
		Use:   "check",
		Short: "Re-run every saved search and report price/stock changes",
		RunE: func(cmd *cobra.Command, args []string) error {
			return vkWatchCheck(flags, cmd)
		},
	}
	check.Flags().IntVar(&watchCheckSize, "size", 10, "Search results to snapshot per watch")
	cmd.AddCommand(check)
	// `vk watch` with no subcommand runs check
	cmd.RunE = check.RunE
	cmd.Flags().IntVar(&watchCheckSize, "size", 10, "Search results to snapshot per watch")
	return cmd
}

var watchCheckSize int

func newVkWatchAddCmd(flags *rootFlags) *cobra.Command {
	var category string
	cmd := &cobra.Command{
		Use:   "add QUERY",
		Short: "Add a search to watch",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			w, err := vkLoadWatches()
			if err != nil {
				return err
			}
			next := 1
			for _, x := range w.Watches {
				if x.ID >= next {
					next = x.ID + 1
				}
			}
			w.Watches = append(w.Watches, vkWatch{
				ID:       next,
				Query:    strings.TrimSpace(strings.Join(args, " ")),
				Category: category,
				Snapshot: map[string]vkSnapshot{},
			})
			if err := vkSaveWatches(w); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "added watch #%d for %q (category=%q)\n", next, strings.Join(args, " "), category)
			return nil
		},
	}
	cmd.Flags().StringVar(&category, "category", "", "Category facet key to scope the watch")
	return cmd
}

func newVkWatchListCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List saved watches",
		RunE: func(cmd *cobra.Command, args []string) error {
			w, err := vkLoadWatches()
			if err != nil {
				return err
			}
			if len(w.Watches) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "no watches saved")
				return nil
			}
			for _, x := range w.Watches {
				fmt.Fprintf(cmd.OutOrStdout(), "#%d  %q", x.ID, x.Query)
				if x.Category != "" {
					fmt.Fprintf(cmd.OutOrStdout(), "  [category=%s]", x.Category)
				}
				fmt.Fprintf(cmd.OutOrStdout(), "  (%d products tracked)\n", len(x.Snapshot))
			}
			return nil
		},
	}
}

func newVkWatchRemoveCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "remove ID",
		Short: "Remove a watch by id",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := strconv.Atoi(args[0])
			if err != nil {
				return fmt.Errorf("watch id must be a number")
			}
			w, err := vkLoadWatches()
			if err != nil {
				return err
			}
			found := false
			out := w.Watches[:0]
			for _, x := range w.Watches {
				if x.ID == id {
					found = true
					continue
				}
				out = append(out, x)
			}
			w.Watches = out
			if err := vkSaveWatches(w); err != nil {
				return err
			}
			if !found {
				return fmt.Errorf("no watch #%d", id)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "removed watch #%d\n", id)
			return nil
		},
	}
}

func vkWatchCheck(flags *rootFlags, cmd *cobra.Command) error {
	w, err := vkLoadWatches()
	if err != nil {
		return err
	}
	if len(w.Watches) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "no watches saved — add one with: verkkokauppa-pp-cli vk watch add \"<query>\"")
		return nil
	}
	c, err := flags.newClient()
	if err != nil {
		return err
	}
	sid := vkSessionID(flags)
	changed := 0
	for _, wt := range w.Watches {
		params := vkSearchParams(wt.Query, wt.Category, sid, watchCheckSize)
		env, err := vkFetchSearchEnv(cmd.Context(), c, params)
		if err != nil {
			fmt.Fprintf(os.Stderr, "watch #%d %q: %v\n", wt.ID, wt.Query, err)
			continue
		}
		prods, err := vkProducts(env)
		if err != nil {
			return err
		}
		pids := make([]string, 0, len(prods))
		for _, p := range prods {
			if p.ID != "" {
				pids = append(pids, p.ID)
			}
		}
		avail, err := vkAvailability(cmd.Context(), c, pids)
		if err != nil {
			return err
		}
		now := map[string]vkSnapshot{}
		for _, p := range prods {
			cur, _ := p.price()
			now[p.ID] = vkSnapshot{Name: p.Attributes.Name, Price: cur, Stock: avail[p.ID]}
		}
		if wt.Snapshot == nil {
			wt.Snapshot = map[string]vkSnapshot{}
		}
		fmt.Fprintf(cmd.OutOrStdout(), "\n[watch #%d] %q%s\n", wt.ID, wt.Query, watchCat(wt.Category))
		fmt.Fprintf(cmd.OutOrStdout(), "  %-7s %-46s %9s  %-20s %s\n", "PID", "NAME", "PRICE", "STOCK", "CHANGE")
		for pid, s := range now {
			prev, ok := wt.Snapshot[pid]
			if !ok {
				prev = vkSnapshot{Price: s.Price, Stock: s.Stock}
			}
			change := "new"
			if ok {
				switch {
				case s.Price < prev.Price:
					change = fmt.Sprintf("▼ %.2f (%.0f%%)", prev.Price-s.Price, (prev.Price-s.Price)/prev.Price*100)
				case s.Price > prev.Price:
					change = fmt.Sprintf("▲ %.2f", s.Price-prev.Price)
				case s.Stock != prev.Stock:
					change = "stock: " + prev.Stock + " -> " + s.Stock
				default:
					change = "—"
				}
			}
			fmt.Fprintf(cmd.OutOrStdout(), "  %-7s %-46s %9.2f  %-20s %s\n", pid, trunc(s.Name, 46), s.Price, s.Stock, change)
			s.PrevPrice, s.PrevStock = prev.Price, prev.Stock
			wt.Snapshot[pid] = s
		}
		for pid := range wt.Snapshot {
			if _, ok := now[pid]; !ok {
				delete(wt.Snapshot, pid)
				changed++
			}
		}
	}
	if err := vkSaveWatches(w); err != nil {
		return err
	}
	fmt.Fprintf(cmd.OutOrStdout(), "\n%d watch(es) with changes; state saved to %s\n", changed, vkWatchesPath())
	return nil
}

func watchCat(c string) string {
	if c == "" {
		return ""
	}
	return " (category=" + c + ")"
}

func vkSessionID(flags *rootFlags) string {
	if v := os.Getenv("VK_SESSION_ID"); v != "" {
		return v
	}
	return vkNewSessionID()
}

func trunc(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n-1]) + "…"
}

func vkPrintJSON(w io.Writer, v interface{}) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}
