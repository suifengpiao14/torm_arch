package tormdb

import (
	"database/sql"
	"os"

	"github.com/jfcote87/sshdb"
	"github.com/jfcote87/sshdb/mysql"
	"github.com/pkg/errors"
	"golang.org/x/crypto/ssh"
)

const (
	PROTOCOL_SSH = "ssh"
)

type SSHConfig struct {
	Address        string
	User           string
	Password       string
	PriviteKeyFile string
}

func (h SSHConfig) Config() (cfg *ssh.ClientConfig, err error) {
	cfg = &ssh.ClientConfig{
		User:            h.User,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Auth:            make([]ssh.AuthMethod, 0),
	}
	if h.Password != "" {
		cfg.Auth = append(cfg.Auth, ssh.Password(h.Password))
		return cfg, nil
	}
	if h.PriviteKeyFile == "" {
		return cfg, nil
	}
	//优先使用keyFile
	k, err := os.ReadFile(h.PriviteKeyFile)
	if err != nil {
		return nil, err
	}
	signer, err := ssh.ParsePrivateKey(k)
	if err != nil {
		return nil, err
	}
	cfg.Auth = append(cfg.Auth, ssh.PublicKeys(signer))
	return cfg, nil
}

func Tunnel(sshCfg SSHConfig, dsn string) (sqlDB *sql.DB, err error) {
	sshConfig, err := sshCfg.Config()
	if err != nil {
		return nil, err
	}
	tunnel, err := sshdb.New(sshConfig, sshCfg.Address)
	if err != nil {
		return nil, err
	}
	tunnel.IgnoreSetDeadlineRequest(true)
	connector, err := tunnel.OpenConnector(mysql.TunnelDriver, dsn)
	if err != nil {
		err = errors.WithMessagef(err, " dsn:%s", dsn)
		return nil, err
	}
	sqlDB = sql.OpenDB(connector)
	return sqlDB, err
}
