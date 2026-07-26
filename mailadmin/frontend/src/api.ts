import axios from 'axios';

const API_URL = '/api';

const api = axios.create({
  baseURL: API_URL,
});

api.interceptors.request.use((config) => {
  const token = localStorage.getItem('token');
  if (token) {
    config.headers.Authorization = `Bearer ${token}`;
  }
  return config;
});

api.interceptors.response.use(
  (response) => response,
  (error) => {
    if (error.response?.status === 401) {
      localStorage.removeItem('token');
      window.location.href = '/login';
    }
    return Promise.reject(error);
  }
);

export interface Domain {
  id: number;
  name: string;
  webmail_enabled: boolean;
  caldav_enabled: boolean;
  carddav_enabled: boolean;
}

export interface User {
  id: number;
  domain_id: number;
  email: string;
  password?: string;
  domain?: string;
  notify_email?: string;
}

export interface Alias {
  id: number;
  domain_id: number;
  source: string;
  destination: string;
  domain?: string;
}

export interface SenderPermission {
  id: number;
  send_as: string;
  login_user: string;
}

export interface APIResponse<T> {
  success: boolean;
  message?: string;
  data?: T;
}

export const auth = {
  login: (username: string, password: string) =>
    api.post<APIResponse<{ token: string }>>('/login', { username, password }),
};

export const domains = {
  list: () => api.get<APIResponse<Domain[]>>('/domains'),
  create: (name: string, webmail_enabled = true) =>
    api.post<APIResponse<Domain>>('/domains', { name, webmail_enabled }),
  update: (id: number, data: Partial<Domain>) =>
    api.put<APIResponse<void>>(`/domains/${id}`, data),
  delete: (id: number) => api.delete<APIResponse<void>>(`/domains/${id}`),
};

export interface DomainAPIKey {
  id: number;
  domain_id: number;
  domain?: string;
  name: string;
  key_prefix: string;
  allowed_sources: string[];
  enabled: boolean;
  last_used_at?: string;
  created_at?: string;
}

// Returned only once when a key is created; `key` is the plaintext secret.
export interface DomainAPIKeyCreated extends DomainAPIKey {
  key: string;
}

export interface APIKeyInput {
  name: string;
  allowed_sources: string[];
  enabled?: boolean;
}

export const apiKeys = {
  list: (domainId: number) =>
    api.get<APIResponse<DomainAPIKey[]>>(`/domains/${domainId}/api-keys`),
  create: (domainId: number, data: APIKeyInput) =>
    api.post<APIResponse<DomainAPIKeyCreated>>(`/domains/${domainId}/api-keys`, data),
  update: (domainId: number, keyId: number, data: APIKeyInput) =>
    api.put<APIResponse<void>>(`/domains/${domainId}/api-keys/${keyId}`, data),
  delete: (domainId: number, keyId: number) =>
    api.delete<APIResponse<void>>(`/domains/${domainId}/api-keys/${keyId}`),
};

export const users = {
  list: () => api.get<APIResponse<User[]>>('/users'),
  create: (user: User) => api.post<APIResponse<User>>('/users', user),
  update: (id: number, user: User) => api.put<APIResponse<void>>(`/users/${id}`, user),
  delete: (id: number) => api.delete<APIResponse<void>>(`/users/${id}`),
  changePassword: (id: number, password: string) =>
    api.put<APIResponse<void>>(`/users/${id}/password`, { password }),
};

export const aliases = {
  list: () => api.get<APIResponse<Alias[]>>('/aliases'),
  create: (alias: Alias) => api.post<APIResponse<Alias>>('/aliases', alias),
  update: (id: number, alias: Alias) => api.put<APIResponse<void>>(`/aliases/${id}`, alias),
  delete: (id: number) => api.delete<APIResponse<void>>(`/aliases/${id}`),
};

export const senderPermissions = {
  list: () => api.get<APIResponse<SenderPermission[]>>('/sender-permissions'),
  create: (perm: SenderPermission) =>
    api.post<APIResponse<SenderPermission>>('/sender-permissions', perm),
  delete: (id: number) => api.delete<APIResponse<void>>(`/sender-permissions/${id}`),
};

export interface MailingList {
  list_id: string;
  name: string;
  display_name: string;
  mail_host: string;
  member_count: number;
  description: string;
  email: string;
}

export interface ListMember {
  email: string;
  display_name: string;
  role: string;
  member_id: string;
}

export interface ListSettings {
  description: string;
  subject_prefix: string;
  archive_policy: string;
  default_member_action: string;
  default_nonmember_action: string;
  digest_enabled: boolean;
  digest_frequency_days: number;
  max_message_size: number;
  subscription_policy: string;
  unsubscription_policy: string;
  admin_immed_notify: boolean;
  admin_notify_mchanges: boolean;
  advertised: boolean;
  allow_list_posts: boolean;
  reply_goes_to_list: boolean;
}

export const mailingLists = {
  list: () => api.get<APIResponse<MailingList[]>>('/lists'),
  create: (data: { name: string; domain: string; description?: string }) =>
    api.post<APIResponse<void>>('/lists', data),
  delete: (listId: string) => api.delete<APIResponse<void>>(`/lists/${listId}`),
  getMembers: (listId: string) => api.get<APIResponse<ListMember[]>>(`/lists/${listId}/members`),
  addMember: (listId: string, data: { email: string; display_name?: string }) =>
    api.post<APIResponse<void>>(`/lists/${listId}/members`, data),
  removeMember: (listId: string, email: string) =>
    api.delete<APIResponse<void>>(`/lists/${listId}/members/${encodeURIComponent(email)}`),
  getSettings: (listId: string) => api.get<APIResponse<ListSettings>>(`/lists/${listId}/settings`),
  updateSettings: (listId: string, settings: Partial<ListSettings>) =>
    api.put<APIResponse<void>>(`/lists/${listId}/settings`, settings),
};

// DNS Check Types
export interface DNSCheckResult {
  check: string;
  status: 'ok' | 'warning' | 'error';
  message: string;
  value?: string;
  help?: string;
  suggested?: string;
}

export interface DomainCheckResponse {
  domain: string;
  checks: DNSCheckResult[];
  overall: 'ok' | 'warning' | 'error';
  server_ip: string;
  hostname: string;
}

export const dnsCheck = {
  check: (domain: string) =>
    api.get<APIResponse<DomainCheckResponse>>(`/dns-check/${domain}`),
  getDkimKey: (domain: string) =>
    api.get<APIResponse<{ key: string; selector: string; domain: string }>>(`/dkim-key/${domain}`),
};

// Client Setup Types
export interface ProtocolConfig {
  server: string;
  port: number;
  port_alt?: number;
  security: string;
  auth_method: string;
}

export interface ClientGuide {
  name: string;
  icon: string;
  platform: string;
  steps: string[];
}

export interface ClientSetupInfo {
  domain: string;
  server_info: {
    hostname: string;
    ip: string;
  };
  imap: ProtocolConfig;
  pop3: ProtocolConfig;
  smtp: ProtocolConfig;
  webmail: {
    url: string;
    enabled: boolean;
  };
  clients: ClientGuide[];
}

export const clientSetup = {
  get: (domain: string) =>
    api.get<APIResponse<ClientSetupInfo>>(`/client-setup/${domain}`),
};

// SSL Certificate Types
export interface SSLStatus {
  domain: string;
  mail_domain: string;
  has_cert: boolean;
  cert_path?: string;
  postfix_configured: boolean;
  dovecot_configured: boolean;
  nginx_webmail: boolean;
  message?: string;
}

export const ssl = {
  getAll: () => api.get<APIResponse<SSLStatus[]>>('/ssl'),
  get: (domain: string) => api.get<APIResponse<SSLStatus>>(`/ssl/${domain}`),
  configure: (domain: string) => api.post<APIResponse<void>>('/ssl/configure', { domain }),
  configureWebmail: (domain: string) => api.post<APIResponse<void>>('/ssl/webmail', { domain }),
};

export default api;
