package api

import (
	"code.cloudfoundry.org/lager"
	"errors"
	"fmt"
	"net"
	"net/http"
	"regexp"
	"strings"
)

var ErrGetP2pUrlFailed = errors.New("failed to get p2p url")

func NewP2pServer(
	logger lager.Logger,
	p2pInterfacePattern *regexp.Regexp,
	p2pInterfaceFamily int,
	p2pStreamPort uint16,
) *P2pServer {
	return &P2pServer{
		p2pInterfacePattern: p2pInterfacePattern,
		p2pInterfaceFamily:  p2pInterfaceFamily,
		p2pStreamPort:       p2pStreamPort,
		logger:              logger,
	}
}

type P2pServer struct {
	p2pInterfacePattern *regexp.Regexp
	p2pInterfaceFamily  int
	p2pStreamPort       uint16

	logger lager.Logger
}

func (server *P2pServer) GetP2pUrl(w http.ResponseWriter, req *http.Request) {
	hLog := server.logger.Session("get-p2p-url")
	hLog.Debug("start")
	defer hLog.Debug("done")

	ifaces, err := net.Interfaces()
	if err != nil {
		RespondWithError(w, ErrGetP2pUrlFailed, http.StatusInternalServerError)
		return
	}

	for _, i := range ifaces {
		if !server.p2pInterfacePattern.MatchString(i.Name) {
			continue
		}

		addrs, err := i.Addrs()
		if err != nil {
			RespondWithError(w, ErrGetP2pUrlFailed, http.StatusInternalServerError)
			return
		}

		for _, addr := range addrs {
			var ip net.IP
			switch v := addr.(type) {
			case *net.IPNet:
				ip = v.IP
			case *net.IPAddr:
				ip = v.IP
			}

			if server.p2pInterfaceFamily == 6 {
				if strings.Contains(ip.String(), ".") {
					continue
				}
			} else { // Default to use IPv4
				if strings.Contains(ip.String(), ":") {
					continue
				}
			}
			hLog.Debug("found-ip", lager.Data{"ip": ip.String()})

			fmt.Fprintf(w, "http://%s:%d", ip.String(), server.p2pStreamPort)
			return
		}
	}

	RespondWithError(w, ErrGetP2pUrlFailed, http.StatusInternalServerError)
}
