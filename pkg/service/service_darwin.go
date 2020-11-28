package service

import (
	"fmt"
	"log/syslog"
	"os"
	"os/exec"
	"os/signal"
	"os/user"
	"text/template"
)

func newService(c *config) (s *darwinLaunchdService, err error) {
	s = &darwinLaunchdService{
		config: c,
	}

	s.logger, err = syslog.New(syslog.LOG_INFO, c.Name)
	if err != nil {
		return nil, err
	}

	return s, nil
}

type darwinLaunchdService struct {
	*config
	logger *syslog.Writer
}

func (s *darwinLaunchdService) getServiceFilePath() string {
	u, err := user.Current()
	if err != nil {
		return "/Library/LaunchDaemons/" + s.Name + ".plist"
	}
	// need sudo?
	return u.HomeDir + "/Library/LaunchAgents/" + s.Name + ".plist"
}

func (s *darwinLaunchdService) Install() error {
	confPath := s.getServiceFilePath()
	_, err = os.Stat(confPath)
	if err == nil {
		return fmt.Errorf("service already exists: %s", confPath)
	}

	f, err := os.Create(confPath)
	if err != nil {
		return err
	}
	defer f.Close()

	dir, err := os.Getwd()
	if err != nil {
		return err
	}
	path := dir + "/" + s.name
	_, err = os.Stat(path)
	if os.IsNotExist(err) {
		return fmt.Errorf("%s executable does not exists, err: %w", s.name, err)
	}

	var to = &struct {
		*config
		Path string

		KeepAlive, RunAtLoad bool
	}{
		config:    s.config,
		Path:      path,
		Args:      s.Args,
		KeepAlive: s.KV.bool("KeepAlive", true),
		RunAtLoad: s.KV.bool("RunAtLoad", false),
	}

	functions := template.FuncMap{
		"bool": func(v bool) string {
			if v {
				return "true"
			}
			return "false"
		},
	}
	var t *template.Template
	t = template.Must(template.New("launchdConfig").Funcs(functions).Parse(launchdConfigPersistent))
	return t.Execute(f, to)
}

func (s *darwinLaunchdService) Remove() error {
	s.Stop()

	confPath := s.getServiceFilePath()
	return os.Remove(confPath)
}

func (s *darwinLaunchdService) Start() error {
	confPath := s.getServiceFilePath()
	cmd := exec.Command("launchctl", "load", confPath)
	return cmd.Run()
}
func (s *darwinLaunchdService) Stop() error {
	confPath := s.getServiceFilePath()
	cmd := exec.Command("launchctl", "unload", confPath)
	return cmd.Run()
}

func (s *darwinLaunchdService) Run(onStart, onStop func() error) error {
	var err error

	err = onStart()
	if err != nil {
		return err
	}

	var sigChan = make(chan os.Signal, 3)

	signal.Notify(sigChan, os.Interrupt, os.Kill)

	<-sigChan

	return onStop()
}

func (s *darwinLaunchdService) Error(format string, a ...interface{}) error {
	return s.logger.Err(fmt.Sprintf(format, a...))
}
func (s *darwinLaunchdService) Warning(format string, a ...interface{}) error {
	return s.logger.Warning(fmt.Sprintf(format, a...))
}
func (s *darwinLaunchdService) Info(format string, a ...interface{}) error {
	// On Darwin syslog.log defaults to loggint >= Notice (see /etc/asl.conf).
	return s.logger.Notice(fmt.Sprintf(format, a...))
}

var launchdConfigPersistent = `<?xml version='1.0' encoding='UTF-8'?>
<!DOCTYPE plist PUBLIC "-//Apple Computer//DTD PLIST 1.0//EN"
"http://www.apple.com/DTDs/PropertyList-1.0.dtd" >
<plist version='1.0'>
<dict>
<key>Label</key><string>{{.Name}}</string>
<key>ProgramArguments</key>
<array>
		<string>{{.Path}}</string>
		<string>{{.Args}}</string>
</array>
<key>KeepAlive</key><{{bool .KeepAlive}}/>
<key>RunAtLoad</key><{{bool .RunAtLoad}}/>
<key>Disabled</key><false/>
</dict>
</plist>
`
