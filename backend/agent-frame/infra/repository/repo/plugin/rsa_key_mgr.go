package plugin

import (
	"context"
	"crypto/rsa"
	"encoding/base64"
	"sync"

	"github.com/jettjia/xiaoqinglong/agent-frame/config"
	"github.com/jettjia/xiaoqinglong/agent-frame/infra/pkg/crypto"

	irepositoryPlugin "github.com/jettjia/xiaoqinglong/agent-frame/domain/irepository/plugin"
)

var _ irepositoryPlugin.IRSAKeyManager = (*RSAKeyManager)(nil)

var (
	rsaKeyMgr     *RSAKeyManager
	rsaKeyMgrOnce sync.Once
)

// RSAKeyManager RSA密钥管理器
type RSAKeyManager struct {
	privateKey *rsa.PrivateKey
	publicKey  *rsa.PublicKey
}

// GetRSAKeyManager 获取RSA密钥管理器单例
func GetRSAKeyManager() *RSAKeyManager {
	rsaKeyMgrOnce.Do(func() {
		rsaKeyMgr = &RSAKeyManager{}
		rsaKeyMgr.init()
	})
	return rsaKeyMgr
}

// init 初始化RSA密钥
func (m *RSAKeyManager) init() {
	// 尝试从配置文件加载密钥
	cfg := config.NewConfig()

	// 尝试从Third.Extra["rsa_private_key"]获取RSA私钥
	if cfg.Third.Extra != nil {
		if rsaKey, ok := cfg.Third.Extra["rsa_private_key"].(string); ok && rsaKey != "" {
			privateKey, err := crypto.ParsePrivateKey([]byte(rsaKey))
			if err == nil {
				m.privateKey = privateKey
				m.publicKey = &privateKey.PublicKey
				return
			}
		}
	}

	// 生成新的RSA密钥对
	keyPair, err := crypto.GenerateRSAKeyPair(2048)
	if err != nil {
		panic("failed to generate RSA key pair: " + err.Error())
	}
	m.privateKey = keyPair.PrivateKey
	m.publicKey = keyPair.PublicKey
}

// GetPublicKey 获取RSA公钥
func (m *RSAKeyManager) GetPublicKey(ctx context.Context) (publicKey string, err error) {
	pemBytes, err := crypto.MarshalRSAPublicKeyPEM(m.publicKey)
	if err != nil {
		return "", err
	}
	return string(pemBytes), nil
}

// EncryptWithRSA 用RSA公钥加密
func (m *RSAKeyManager) EncryptWithRSA(ctx context.Context, data string) (encrypted string, err error) {
	encryptedB64, err := crypto.EncryptRSAOAEPBase64([]byte(data), m.publicKey)
	if err != nil {
		return "", err
	}
	return encryptedB64, nil
}

// DecryptWithRSA 用RSA私钥解密
func (m *RSAKeyManager) DecryptWithRSA(ctx context.Context, encrypted string) (data string, err error) {
	decrypted, err := crypto.DecryptRSAOAEPBase64(encrypted, m.privateKey)
	if err != nil {
		return "", err
	}
	return string(decrypted), nil
}

// DecryptToken 解密token（使用AES + RSA双重加密）
// encryptedToken: AES加密后的token
// encryptedAES: RSA加密后的AES密钥
func (m *RSAKeyManager) DecryptToken(ctx context.Context, encryptedToken, encryptedAES string) (token string, err error) {
	// 1. 用RSA私钥解密AES密钥
	aesKeyB64, err := m.DecryptWithRSA(ctx, encryptedAES)
	if err != nil {
		return "", err
	}

	// 2. base64解码AES密钥
	aesKey, err := base64.StdEncoding.DecodeString(aesKeyB64)
	if err != nil {
		return "", err
	}

	// 3. 用AES密钥解密token
	decrypted, err := crypto.DecryptAESGCMBase64(encryptedToken, aesKey)
	if err != nil {
		return "", err
	}

	return string(decrypted), nil
}

// EncryptToken 加密token（使用AES + RSA双重加密）
func (m *RSAKeyManager) EncryptToken(ctx context.Context, token string) (encryptedToken, encryptedAES string, err error) {
	// 1. 生成随机AES密钥
	aesKey, err := crypto.GenerateAESKey(32) // 256位
	if err != nil {
		return "", "", err
	}

	// 2. 用AES密钥加密token
	encryptedToken, err = crypto.EncryptAESGCMBase64([]byte(token), aesKey)
	if err != nil {
		return "", "", err
	}

	// 3. 用RSA公钥加密AES密钥
	aesKeyB64 := base64.StdEncoding.EncodeToString(aesKey)
	encryptedAES, err = m.EncryptWithRSA(ctx, aesKeyB64)
	if err != nil {
		return "", "", err
	}

	return encryptedToken, encryptedAES, nil
}