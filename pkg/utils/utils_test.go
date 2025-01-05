package utils

import (
	"net"
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

func TestCompareNets(t *testing.T) {
	Convey("compare 2 nets", t, func() {

		Convey("compare 2 same nets in same order", func() {
			old := []net.IP{
				net.ParseIP("192.168.0.1"),
				net.ParseIP("192.168.0.2"),
			}
			new := []net.IP{
				net.ParseIP("192.168.0.1"),
				net.ParseIP("192.168.0.2"),
			}
			same := CompareNets(old, new)
			Convey("result", func() {
				So(same, ShouldEqual, true)
			})
		})

		Convey("compare 2 same nets in different order", func() {
			old := []net.IP{
				net.ParseIP("192.168.0.1"),
				net.ParseIP("192.168.0.2"),
			}
			new := []net.IP{
				net.ParseIP("192.168.0.2"),
				net.ParseIP("192.168.0.1"),
			}
			same := CompareNets(old, new)
			Convey("result", func() {
				So(same, ShouldEqual, true)
			})
		})

		Convey("compare 2 different nets with same length", func() {
			old := []net.IP{
				net.ParseIP("192.168.0.1"),
				net.ParseIP("192.168.0.2"),
			}
			new := []net.IP{
				net.ParseIP("192.168.0.1"),
				net.ParseIP("192.168.0.3"),
			}
			same := CompareNets(old, new)
			Convey("result", func() {
				So(same, ShouldEqual, false)
			})
		})

		Convey("compare 2 different nets with different length", func() {
			old := []net.IP{
				net.ParseIP("192.168.0.1"),
				net.ParseIP("192.168.0.2"),
			}
			new := []net.IP{
				net.ParseIP("192.168.0.1"),
				net.ParseIP("192.168.0.2"),
				net.ParseIP("192.168.0.3"),
			}
			same := CompareNets(old, new)
			Convey("result", func() {
				So(same, ShouldEqual, false)
			})
		})
	})
}
