package chart

type Summary struct {
	Modules *map[string]int `json:"modules"`
	Edges *[]string `json:"edges"`
	Names *[]string `json:"names"`
}

func NewSummary() *Summary {
	modules := map[string]int{}
	edges := []string{}
	names := []string{}

	summary := &Summary{
		Modules: &modules,
		Edges: &edges,
		Names: &names,
	}

	return summary
}

func (s *Summary) Reset() {
	modules := map[string]int{}
	edges := []string{}
	names := []string{}

  s.Modules = &modules
	s.Edges = &edges
	s.Names = &names
}

func (s *Summary) AddModule(module string) {
	modules := *s.Modules

	_, exists := modules[module]
	if exists {
		modules[module]++
	} else {
		modules[module] = 1
	}
}

func (s *Summary) AddEdge(edge string) {
	*s.Edges = append(*s.Edges, edge)
}

func (s *Summary) AddName(name string) {
	*s.Names = append(*s.Names, name)
}
