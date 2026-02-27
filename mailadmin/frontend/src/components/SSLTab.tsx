import { useState, useEffect } from 'react';
import { ssl } from '../api';
import type { SSLStatus } from '../api';

function SSLTab() {
  const [sslList, setSSLList] = useState<SSLStatus[]>([]);
  const [loading, setLoading] = useState(true);
  const [configuring, setConfiguring] = useState<string | null>(null);
  const [error, setError] = useState('');
  const [success, setSuccess] = useState('');

  const fetchSSLStatus = async () => {
    setLoading(true);
    try {
      const response = await ssl.getAll();
      if (response.data.success) {
        setSSLList(response.data.data || []);
      }
    } catch (err) {
      setError('SSL durumu alinamadi');
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    fetchSSLStatus();
  }, []);

  const handleConfigure = async (domain: string) => {
    if (!confirm(`${domain} icin SSL sertifikasi olusturulacak ve yapilandirilacak. Devam etmek istiyor musunuz?`)) {
      return;
    }

    setConfiguring(domain);
    setError('');
    setSuccess('');

    try {
      const response = await ssl.configure(domain);
      if (response.data.success) {
        setSuccess(response.data.message || `${domain} icin SSL basariyla yapilandirildi`);
        await fetchSSLStatus();
      } else {
        setError(response.data.message || 'SSL yapilandirilamadi');
      }
    } catch (err: any) {
      setError(err.response?.data?.message || 'SSL yapilandirilamadi');
    } finally {
      setConfiguring(null);
    }
  };

  const handleConfigureWebmail = async (domain: string) => {
    if (!confirm(`${domain} icin nginx webmail yapilandirmasi olusturulacak. Devam etmek istiyor musunuz?`)) {
      return;
    }

    setConfiguring(domain + '-webmail');
    setError('');
    setSuccess('');

    try {
      const response = await ssl.configureWebmail(domain);
      if (response.data.success) {
        setSuccess(response.data.message || `${domain} icin webmail basariyla yapilandirildi`);
        await fetchSSLStatus();
      } else {
        setError(response.data.message || 'Webmail yapilandirilamadi');
      }
    } catch (err: any) {
      setError(err.response?.data?.message || 'Webmail yapilandirilamadi');
    } finally {
      setConfiguring(null);
    }
  };

  const getStatusIcon = (status: SSLStatus) => {
    if (status.has_cert && status.postfix_configured && status.dovecot_configured && status.nginx_webmail) {
      return <span className="status-icon ok" title="Tamam">✓</span>;
    } else if (status.has_cert) {
      return <span className="status-icon warning" title="Yapilandirma eksik">⚠</span>;
    } else {
      return <span className="status-icon error" title="Sertifika yok">✗</span>;
    }
  };

  const getStatusClass = (status: SSLStatus) => {
    if (status.has_cert && status.postfix_configured && status.dovecot_configured && status.nginx_webmail) {
      return 'ssl-ok';
    } else if (status.has_cert) {
      return 'ssl-warning';
    } else {
      return 'ssl-error';
    }
  };

  const isSSLConfigured = (status: SSLStatus) => {
    return status.has_cert && status.postfix_configured && status.dovecot_configured;
  };

  const isFullyConfigured = (status: SSLStatus) => {
    return status.has_cert && status.postfix_configured && status.dovecot_configured && status.nginx_webmail;
  };

  if (loading) {
    return <div className="loading">Yukleniyor...</div>;
  }

  return (
    <div className="ssl-tab">
      <div className="tab-header">
        <h2>SSL Sertifika Yonetimi</h2>
        <button className="refresh-btn" onClick={fetchSSLStatus}>
          ↻ Yenile
        </button>
      </div>

      {error && <div className="error-message">{error}</div>}
      {success && <div className="success-message">{success}</div>}

      <div className="ssl-info">
        <p>
          Mail servisleri (IMAP/SMTP) icin her domain'in <code>mail.domain.com</code> adresi
          icin gecerli bir SSL sertifikasina ihtiyaci vardir.
        </p>
      </div>

      <table className="ssl-table">
        <thead>
          <tr>
            <th>Domain</th>
            <th>Mail Adresi</th>
            <th>Sertifika</th>
            <th>Postfix</th>
            <th>Dovecot</th>
            <th>Webmail</th>
            <th>Durum</th>
            <th>Islem</th>
          </tr>
        </thead>
        <tbody>
          {sslList.map((status) => (
            <tr key={status.domain} className={getStatusClass(status)}>
              <td>{status.domain}</td>
              <td><code>{status.mail_domain}</code></td>
              <td className="status-cell">
                {status.has_cert ? (
                  <span className="badge ok">Mevcut</span>
                ) : (
                  <span className="badge error">Yok</span>
                )}
              </td>
              <td className="status-cell">
                {status.postfix_configured ? (
                  <span className="badge ok">✓</span>
                ) : (
                  <span className="badge error">✗</span>
                )}
              </td>
              <td className="status-cell">
                {status.dovecot_configured ? (
                  <span className="badge ok">✓</span>
                ) : (
                  <span className="badge error">✗</span>
                )}
              </td>
              <td className="status-cell">
                {status.nginx_webmail ? (
                  <span className="badge ok">✓</span>
                ) : (
                  <span className="badge error">✗</span>
                )}
              </td>
              <td>
                {getStatusIcon(status)}
                <span className="status-text">{status.message}</span>
              </td>
              <td className="action-cell">
                {!isSSLConfigured(status) && (
                  <button
                    className="configure-btn"
                    onClick={() => handleConfigure(status.domain)}
                    disabled={configuring !== null}
                  >
                    {configuring === status.domain ? 'Yapilandiriliyor...' : 'SSL Yapilandir'}
                  </button>
                )}
                {isSSLConfigured(status) && !status.nginx_webmail && (
                  <button
                    className="configure-btn webmail-btn"
                    onClick={() => handleConfigureWebmail(status.domain)}
                    disabled={configuring !== null}
                  >
                    {configuring === status.domain + '-webmail' ? 'Yapilandiriliyor...' : 'Webmail Yapilandir'}
                  </button>
                )}
                {isFullyConfigured(status) && (
                  <span className="configured-text">Hazir</span>
                )}
              </td>
            </tr>
          ))}
        </tbody>
      </table>

      {sslList.length === 0 && (
        <div className="empty-state">
          Henuz domain eklenmemis.
        </div>
      )}

      <div className="ssl-legend">
        <h4>Aciklama</h4>
        <ul>
          <li><span className="badge ok">✓</span> Yapilandirilmis</li>
          <li><span className="badge error">✗</span> Yapilandirilmamis</li>
          <li><strong>Sertifika:</strong> Let's Encrypt sertifikasi mevcut mu</li>
          <li><strong>Postfix:</strong> SMTP sunucu SNI yapilandirmasi</li>
          <li><strong>Dovecot:</strong> IMAP/POP3 sunucu SSL yapilandirmasi</li>
          <li><strong>Webmail:</strong> Nginx Groupware proxy yapilandirmasi</li>
        </ul>
      </div>
    </div>
  );
}

export default SSLTab;
