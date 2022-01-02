package plot

import (
	"gonum.org/v1/gonum/graph"
	"gonum.org/v1/gonum/graph/simple"
	"gonum.org/v1/gonum/graph/traverse"
)

type OptimisationFunc func(p Canvas, d Drawing) (Drawing, error)

func OptimiseLinearLineOrder(p Canvas, d Drawing) (Drawing, error) {
	// basic optimization: if the order of line segments is inverted, make it a continuous line
	g, objs, residual := buildGraph(d)

	var res = make([]Drawable, 0, len(d))

	// Start with all source nodes and go depth-first until we're at the end of the path, or find a node we've seen before.
	// We start with source nodes only to ensure we get the longest possible paths.
	nodes := g.Nodes()
	var dfs traverse.DepthFirst
	dfs.Visit = func(n graph.Node) {
		res = append(res, objs[n.ID()])
	}
	for nodes.Next() {
		n := nodes.Node()
		if g.To(n.ID()).Len() > 0 {
			// not a source node
			continue
		}

		dfs.Walk(g, n, nil)
	}

	// TODO(cw): this approach "swallows" circles in the graph

	res = append(res, residual...)
	return res, nil
}

func buildGraph(d Drawing) (g graph.Directed, objs map[int64]Drawable, residual []Drawable) {
	var (
		res      = simple.NewDirectedGraph()
		startIdx = make(map[XY][]graph.Node)
		objIdx   = make(map[int64]Drawable)
	)
	for _, e := range d {
		switch elem := e.(type) {
		case Line:
			nodes, ok := startIdx[elem.Start]
			if !ok || len(nodes) == 0 {
				n := res.NewNode()
				startIdx[elem.Start] = append(startIdx[elem.Start], n)
				objIdx[n.ID()] = elem
				res.AddNode(n)
			}
		default:
			residual = append(residual, e)
		}
	}

	for _, e := range d {
		switch elem := e.(type) {
		case Line:
			st := startIdx[elem.Start]
			for _, et := range startIdx[elem.End] {
				if st[0] == et {
					continue
				}
				res.SetEdge(res.NewEdge(st[0], et))
			}
		default:
			residual = append(residual, e)
		}
	}
	return res, objIdx, residual
}
