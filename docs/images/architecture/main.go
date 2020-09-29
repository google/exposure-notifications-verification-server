package main

import (
	"log"

	"github.com/blushft/go-diagrams/diagram"
	"github.com/blushft/go-diagrams/nodes/gcp"
	"github.com/blushft/go-diagrams/nodes/generic"
)

func main() {
	d, err := diagram.New(diagram.Filename("app"), diagram.Label("EN Verification Server Architecture"), diagram.Direction("LR"))
	if err != nil {
		log.Fatal(err)
	}

	laptop := generic.Device.Tablet(diagram.NodeLabel("laptop"))
	phone := generic.Device.Mobile(diagram.NodeLabel("phone"))

	serverDNS := gcp.Network.Dns(diagram.NodeLabel(" encv.org"))
	apiDNS := gcp.Network.Dns(diagram.NodeLabel(" apiserver.encv.org"))
	adminDNS := gcp.Network.Dns(diagram.NodeLabel(" adminapi.encv.org"))

	lb := gcp.Network.LoadBalancing(diagram.NodeLabel("LB"))

	redirectDNS := gcp.Network.Dns(diagram.NodeLabel("*.en.express"))
	redirectLB := gcp.Network.LoadBalancing(diagram.NodeLabel("Redirect LB"))

	cache := gcp.Database.Memorystore(diagram.NodeLabel("Redis"))
	db := gcp.Database.Sql(diagram.NodeLabel("Postgres"))

	scheduler := gcp.Devtools.Scheduler(diagram.NodeLabel("Cloud Scheduler"))
	cleanup := gcp.Compute.Run(diagram.NodeLabel("Cleanup"))

	u := diagram.NewGroup("Users").Label("Users").Add(laptop, phone)

	dc := diagram.NewGroup("GCP").Label("GCP").Add(redirectLB, lb)

	dc.NewGroup("data").
		Label("Data Stores").
		Add(db, cache)

	dc.NewGroup("services").
		Label("Verification").
		Add(
			gcp.Compute.Run(diagram.NodeLabel("API Server")),
			gcp.Compute.Run(diagram.NodeLabel("Admin API")),
			gcp.Compute.Run(diagram.NodeLabel("UI Server")),
		).
		ConnectAllFrom(lb.ID(), diagram.Forward()).
		ConnectAllTo(db.ID(), diagram.Forward()).
		ConnectAllTo(cache.ID(), diagram.Forward())

	dc.NewGroup("enx-redirect").
		Label("Redirect Server").
		Add(gcp.Compute.Run(diagram.NodeLabel("Redirect"))).
		ConnectAllFrom(redirectLB.ID(), diagram.Forward()).
		ConnectAllTo(db.ID(), diagram.Forward()).
		ConnectAllTo(cache.ID(), diagram.Forward())

	dc.NewGroup("retention").
		Label("Cleanup").
		Add(cleanup, scheduler).
		Connect(scheduler, cleanup, diagram.Forward()).
		Connect(cleanup, db, diagram.Forward())

	d.
		Connect(redirectDNS, redirectLB, diagram.Forward()).
		Connect(apiDNS, lb, diagram.Forward()).
		Connect(adminDNS, lb, diagram.Forward()).
		Connect(serverDNS, lb, diagram.Forward()).
		Connect(laptop, adminDNS, diagram.Forward()).
		Connect(laptop, serverDNS, diagram.Forward()).
		Connect(phone, apiDNS, diagram.Forward()).
		Connect(phone, redirectDNS, diagram.Forward()).
		Group(u).
		Group(dc)

	if err := d.Render(); err != nil {
		log.Fatal(err)
	}
}
