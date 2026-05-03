package chart

// Summary contains some stats gathered whilst generating a chart.
type Summary struct {
	Modules *map[string]int `json:"modules"`
	Edges   *[]string       `json:"edges"`
	Names   *[]string       `json:"names"`
}

// NewSummary returns a Summary instance.
func NewSummary() *Summary {
	modules := map[string]int{}
	edges := []string{}
	names := []string{}

	summary := &Summary{
		Modules: &modules,
		Edges:   &edges,
		Names:   &names,
	}

	return summary
}

// Reset empties all the gathered summary so that the chart generation can be re-run.
func (s *Summary) Reset() {
	modules := map[string]int{}
	edges := []string{}
	names := []string{}

	s.Modules = &modules
	s.Edges = &edges
	s.Names = &names
}

// AddModule increments module occurrence in the summary.
func (s *Summary) AddModule(module string) {
	modules := *s.Modules

	_, exists := modules[module]
	if exists {
		modules[module]++
	} else {
		modules[module] = 1
	}
}

// AddEdge adds an resource name edge element to the summary.
func (s *Summary) AddEdge(edge string) {
	*s.Edges = append(*s.Edges, edge)
}

// AddName adds a resource field name to the summary.
func (s *Summary) AddName(name string) {
	*s.Names = append(*s.Names, name)
}
