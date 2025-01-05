package syncer

import (
	"net"
	"testing"

	"github.com/moresec-io/conduit/pkg/proto"
	. "github.com/smartystreets/goconvey/convey"
)

func TestCompareConduits(t *testing.T) {
	Convey("compare 2 conduits", t, func() {

		Convey("compare 2 same conduits", func() {
			old := []proto.Conduit{
				{
					MachineID: "1",
					Network:   "tcp",
					Addr:      "192.168.0.1:443",
					IPs: []net.IP{
						net.ParseIP("192.168.0.1"),
					},
				},
			}
			new := []proto.Conduit{
				{
					MachineID: "1",
					Network:   "tcp",
					Addr:      "192.168.0.1:443",
					IPs: []net.IP{
						net.ParseIP("192.168.0.1"),
					},
				},
			}
			removes, adds := compareConduits(old, new)
			Convey("result", func() {
				So(len(removes), ShouldEqual, 0)
				So(len(adds), ShouldEqual, 0)
			})
		})

		Convey("compare 2 ip different conduits", func() {
			old := []proto.Conduit{
				{
					MachineID: "1",
					Network:   "tcp",
					Addr:      "192.168.0.1:443",
					IPs: []net.IP{
						net.ParseIP("192.168.0.1"),
					},
				},
			}
			new := []proto.Conduit{
				{
					MachineID: "1",
					Network:   "tcp",
					Addr:      "192.168.0.1:443",
					IPs: []net.IP{
						net.ParseIP("192.168.1.1"),
					},
				},
			}
			removes, adds := compareConduits(old, new)
			Convey("result", func() {
				So(len(removes), ShouldEqual, 1)
				So(len(adds), ShouldEqual, 1)
			})
		})

		Convey("compare 2 id different conduits", func() {
			old := []proto.Conduit{
				{
					MachineID: "1",
					Network:   "tcp",
					Addr:      "192.168.0.1:443",
					IPs: []net.IP{
						net.ParseIP("192.168.0.1"),
					},
				},
			}
			new := []proto.Conduit{
				{
					MachineID: "2",
					Network:   "tcp",
					Addr:      "192.168.0.1:443",
					IPs: []net.IP{
						net.ParseIP("192.168.0.2"),
					},
				},
			}
			removes, adds := compareConduits(old, new)
			Convey("result", func() {
				So(len(removes), ShouldEqual, 1)
				So(len(adds), ShouldEqual, 1)
			})
		})

		Convey("compare 2 id different conduits, new include old", func() {
			old := []proto.Conduit{
				{
					MachineID: "1",
					Network:   "tcp",
					Addr:      "192.168.0.1:443",
					IPs: []net.IP{
						net.ParseIP("192.168.0.1"),
					},
				},
			}
			new := []proto.Conduit{
				{
					MachineID: "1",
					Network:   "tcp",
					Addr:      "192.168.0.1:443",
					IPs: []net.IP{
						net.ParseIP("192.168.0.1"),
					},
				},
				{
					MachineID: "2",
					Network:   "tcp",
					Addr:      "192.168.0.1:443",
					IPs: []net.IP{
						net.ParseIP("192.168.0.2"),
					},
				},
			}
			removes, adds := compareConduits(old, new)
			Convey("result", func() {
				So(len(removes), ShouldEqual, 0)
				So(len(adds), ShouldEqual, 1)
			})
		})

		Convey("compare 2 id different conduits, old include new", func() {
			old := []proto.Conduit{
				{
					MachineID: "1",
					Network:   "tcp",
					Addr:      "192.168.0.1:443",
					IPs: []net.IP{
						net.ParseIP("192.168.0.1"),
					},
				},
				{
					MachineID: "2",
					Network:   "tcp",
					Addr:      "192.168.0.1:443",
					IPs: []net.IP{
						net.ParseIP("192.168.0.2"),
					},
				},
			}
			new := []proto.Conduit{
				{
					MachineID: "2",
					Network:   "tcp",
					Addr:      "192.168.0.1:443",
					IPs: []net.IP{
						net.ParseIP("192.168.0.2"),
					},
				},
			}
			removes, adds := compareConduits(old, new)
			Convey("result", func() {
				So(len(removes), ShouldEqual, 1)
				So(len(adds), ShouldEqual, 0)
			})
		})
	})
}
