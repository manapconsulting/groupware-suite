import axios from 'axios';

const API_URL = '/mail/api';

const api = axios.create({
  baseURL: API_URL,
});

api.interceptors.request.use((config) => {
  const token = localStorage.getItem('webmail_token');
  if (token) {
    config.headers.Authorization = `Bearer ${token}`;
  }
  return config;
});

api.interceptors.response.use(
  (response) => response,
  (error) => {
    if (error.response?.status === 401) {
      localStorage.removeItem('webmail_token');
      localStorage.removeItem('webmail_email');
      window.location.reload();
    }
    return Promise.reject(error);
  }
);

export interface Folder {
  name: string;
  delimiter: string;
  unread: number;
  total: number;
}

export interface MessageSummary {
  uid: number;
  subject: string;
  from: string;
  to: string;
  date: string;
  size: number;
  seen: boolean;
  flagged: boolean;
  has_attachment: boolean;
}

export interface MessageDetail {
  uid: number;
  subject: string;
  from: string;
  to: string;
  cc: string;
  date: string;
  text_body: string;
  html_body: string;
  attachments: Attachment[];
  seen: boolean;
  flagged: boolean;
}

export interface Attachment {
  index: number;
  filename: string;
  content_type: string;
  size: number;
}

export interface APIResponse<T> {
  success: boolean;
  message?: string;
  data?: T;
}

export interface MessagesResponse {
  messages: MessageSummary[];
  total: number;
  page: number;
  pages: number;
}

export const auth = {
  login: (email: string, password: string) =>
    api.post<APIResponse<{ token: string; email: string }>>('/login', { email, password }),
};

export const folders = {
  list: () => api.get<APIResponse<Folder[]>>('/folders'),
};

export const messages = {
  list: (folder: string, page = 1) =>
    api.get<APIResponse<MessagesResponse>>(`/messages/${encodeURIComponent(folder)}?page=${page}`),
  get: (folder: string, uid: number) =>
    api.get<APIResponse<MessageDetail>>(`/messages/${encodeURIComponent(folder)}/${uid}`),
  delete: (folder: string, uid: number) =>
    api.delete<APIResponse<void>>(`/messages/${encodeURIComponent(folder)}/${uid}`),
  move: (folder: string, uid: number, destination: string) =>
    api.post<APIResponse<void>>(`/messages/${encodeURIComponent(folder)}/${uid}/move`, { destination }),
  updateFlags: (folder: string, uid: number, flags: { seen?: boolean; flagged?: boolean }) =>
    api.put<APIResponse<void>>(`/messages/${encodeURIComponent(folder)}/${uid}/flags`, flags),
  send: (data: {
    to: string[];
    cc?: string[];
    bcc?: string[];
    subject: string;
    body: string;
    is_html: boolean;
    forward_attachments?: {
      source_folder: string;
      source_uid: number;
      indexes: number[];
    };
  }) => api.post<APIResponse<void>>('/send', data),
};

export const getAttachmentUrl = (folder: string, uid: number, index: number) => {
  const token = localStorage.getItem('webmail_token');
  return `${API_URL}/messages/${encodeURIComponent(folder)}/${uid}/attachments/${index}?token=${encodeURIComponent(token || '')}`;
};

// Calendar Types
export interface Calendar {
  id: number;
  name: string;
  color: string;
  description?: string;
}

export interface CalendarEvent {
  id?: number;
  calendar_id?: number;
  uid?: string;
  summary: string;
  description?: string;
  location?: string;
  start_time: string;
  end_time: string;
  all_day: boolean;
}

// Contact Types
export interface Addressbook {
  id: number;
  name: string;
  description?: string;
}

export interface Contact {
  id?: number;
  addressbook_id?: number;
  uid?: string;
  full_name: string;
  email?: string;
  phone?: string;
}

export const calendars = {
  list: () => api.get<APIResponse<Calendar[]>>('/calendars'),
};

export const events = {
  list: (start?: string, end?: string) => {
    let url = '/events';
    if (start && end) {
      url += `?start=${encodeURIComponent(start)}&end=${encodeURIComponent(end)}`;
    }
    return api.get<APIResponse<CalendarEvent[]>>(url);
  },
  create: (event: CalendarEvent) => api.post<APIResponse<CalendarEvent>>('/events', event),
  update: (id: number, event: CalendarEvent) => api.put<APIResponse<void>>(`/events/${id}`, event),
  delete: (id: number) => api.delete<APIResponse<void>>(`/events/${id}`),
};

export const addressbooks = {
  list: () => api.get<APIResponse<Addressbook[]>>('/addressbooks'),
};

export const contacts = {
  list: (query?: string) => {
    let url = '/contacts';
    if (query) {
      url += `?q=${encodeURIComponent(query)}`;
    }
    return api.get<APIResponse<Contact[]>>(url);
  },
  create: (contact: Contact) => api.post<APIResponse<Contact>>('/contacts', contact),
  update: (id: number, contact: Contact) => api.put<APIResponse<void>>(`/contacts/${id}`, contact),
  delete: (id: number) => api.delete<APIResponse<void>>(`/contacts/${id}`),
};

// Shared Calendar Types
export interface SharedCalendar {
  id: number;
  name: string;
  color: string;
  description?: string;
  owner_id: number;
  owner_email?: string;
  role?: string;
}

export interface CalendarMember {
  id: number;
  user_id: number;
  email: string;
  role: string;
}

export interface SharedEvent {
  id?: number;
  calendar_id?: number;
  uid?: string;
  summary: string;
  description?: string;
  location?: string;
  start_time: string;
  end_time: string;
  all_day: boolean;
  created_by?: number;
  creator_email?: string;
}

export interface FreeBusySlot {
  start_time: string;
  end_time: string;
  status: string;
}

export interface UserFreeBusy {
  user_id: number;
  email: string;
  slots: FreeBusySlot[];
}

export interface AvailableSlot {
  start_time: string;
  end_time: string;
}

export interface DomainUser {
  id: number;
  email: string;
}

export const sharedCalendars = {
  list: () => api.get<APIResponse<SharedCalendar[]>>('/shared-calendars'),
  create: (calendar: Partial<SharedCalendar>) => api.post<APIResponse<SharedCalendar>>('/shared-calendars', calendar),
  update: (id: number, calendar: Partial<SharedCalendar>) => api.put<APIResponse<void>>(`/shared-calendars/${id}`, calendar),
  delete: (id: number) => api.delete<APIResponse<void>>(`/shared-calendars/${id}`),
  getMembers: (id: number) => api.get<APIResponse<CalendarMember[]>>(`/shared-calendars/${id}/members`),
  addMember: (id: number, email: string, role: string) =>
    api.post<APIResponse<void>>(`/shared-calendars/${id}/members`, { email, role }),
  removeMember: (id: number, memberId: number) =>
    api.delete<APIResponse<void>>(`/shared-calendars/${id}/members/${memberId}`),
  getEvents: (id: number) => api.get<APIResponse<SharedEvent[]>>(`/shared-calendars/${id}/events`),
  createEvent: (id: number, event: SharedEvent) =>
    api.post<APIResponse<SharedEvent>>(`/shared-calendars/${id}/events`, event),
  updateEvent: (id: number, eventId: number, event: SharedEvent) =>
    api.put<APIResponse<void>>(`/shared-calendars/${id}/events/${eventId}`, event),
  deleteEvent: (id: number, eventId: number) =>
    api.delete<APIResponse<void>>(`/shared-calendars/${id}/events/${eventId}`),
  getFreeBusy: (id: number, start?: string, end?: string) => {
    let url = `/shared-calendars/${id}/freebusy`;
    if (start && end) {
      url += `?start=${encodeURIComponent(start)}&end=${encodeURIComponent(end)}`;
    }
    return api.get<APIResponse<UserFreeBusy[]>>(url);
  },
  findAvailableTime: (id: number, params: {
    start_date: string;
    end_date: string;
    duration_mins: number;
    workday_start?: number;
    workday_end?: number;
  }) => api.post<APIResponse<AvailableSlot[]>>(`/shared-calendars/${id}/find-time`, params),
};

export const domainUsers = {
  list: () => api.get<APIResponse<DomainUser[]>>('/domain-users'),
};

export default api;
