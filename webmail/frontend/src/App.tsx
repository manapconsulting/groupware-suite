import { useState, useEffect } from 'react';
import { auth, folders, messages, getAttachmentUrl, events, contacts, sharedCalendars, domainUsers } from './api';
import type { Folder, MessageSummary, MessageDetail, CalendarEvent, Contact, SharedCalendar, CalendarMember, SharedEvent, UserFreeBusy, AvailableSlot, DomainUser } from './api';

type ActiveTab = 'mail' | 'calendar' | 'contacts';
type CalendarView = 'personal' | 'group';

function App() {
  const [token, setToken] = useState(localStorage.getItem('webmail_token'));
  const [email, setEmail] = useState(localStorage.getItem('webmail_email') || '');
  const [password, setPassword] = useState('');
  const [loginError, setLoginError] = useState('');
  const [loading, setLoading] = useState(false);
  const [activeTab, setActiveTab] = useState<ActiveTab>('mail');

  // Mail state
  const [folderList, setFolderList] = useState<Folder[]>([]);
  const [selectedFolder, setSelectedFolder] = useState('INBOX');
  const [messageList, setMessageList] = useState<MessageSummary[]>([]);
  const [selectedMessage, setSelectedMessage] = useState<MessageDetail | null>(null);
  const [page, setPage] = useState(1);
  const [totalPages, setTotalPages] = useState(1);
  const [loadingMessages, setLoadingMessages] = useState(false);

  // Compose state
  const [showCompose, setShowCompose] = useState(false);
  const [composeTo, setComposeTo] = useState('');
  const [composeCc, setComposeCc] = useState('');
  const [composeSubject, setComposeSubject] = useState('');
  const [composeBody, setComposeBody] = useState('');
  const [sending, setSending] = useState(false);
  const [replyAttachments, setReplyAttachments] = useState<{index: number; filename: string; selected: boolean}[]>([]);
  const [replySource, setReplySource] = useState<{folder: string; uid: number} | null>(null);

  // Calendar state
  const [eventList, setEventList] = useState<CalendarEvent[]>([]);
  const [selectedDate, setSelectedDate] = useState(new Date());
  const [showEventModal, setShowEventModal] = useState(false);
  const [editingEvent, setEditingEvent] = useState<CalendarEvent | null>(null);

  // Contacts state
  const [contactList, setContactList] = useState<Contact[]>([]);
  const [contactSearch, setContactSearch] = useState('');
  const [showContactModal, setShowContactModal] = useState(false);
  const [editingContact, setEditingContact] = useState<Contact | null>(null);

  // Shared Calendar state
  const [calendarView, setCalendarView] = useState<CalendarView>('personal');
  const [sharedCalendarList, setSharedCalendarList] = useState<SharedCalendar[]>([]);
  const [selectedSharedCalendar, setSelectedSharedCalendar] = useState<SharedCalendar | null>(null);
  const [sharedEventList, setSharedEventList] = useState<SharedEvent[]>([]);
  const [calendarMembers, setCalendarMembers] = useState<CalendarMember[]>([]);
  const [showSharedCalendarModal, setShowSharedCalendarModal] = useState(false);
  const [editingSharedCalendar, setEditingSharedCalendar] = useState<SharedCalendar | null>(null);
  const [showMembersModal, setShowMembersModal] = useState(false);
  const [showFreeBusyModal, setShowFreeBusyModal] = useState(false);
  const [freeBusyData, setFreeBusyData] = useState<UserFreeBusy[]>([]);
  const [showFindTimeModal, setShowFindTimeModal] = useState(false);
  const [availableSlots, setAvailableSlots] = useState<AvailableSlot[]>([]);
  const [domainUserList, setDomainUserList] = useState<DomainUser[]>([]);
  const [newMemberEmail, setNewMemberEmail] = useState('');
  const [newMemberRole, setNewMemberRole] = useState('viewer');
  const [findTimeParams, setFindTimeParams] = useState({
    start_date: new Date().toISOString().slice(0, 10),
    end_date: new Date(Date.now() + 7 * 24 * 60 * 60 * 1000).toISOString().slice(0, 10),
    duration_mins: 60,
    workday_start: 9,
    workday_end: 18
  });
  const [showSharedEventModal, setShowSharedEventModal] = useState(false);
  const [editingSharedEvent, setEditingSharedEvent] = useState<SharedEvent | null>(null);

  useEffect(() => {
    if (token) {
      loadFolders();
    }
  }, [token]);

  useEffect(() => {
    if (token && selectedFolder) {
      loadMessages();
    }
  }, [token, selectedFolder, page]);

  const handleLogin = async (e: React.FormEvent) => {
    e.preventDefault();
    setLoading(true);
    setLoginError('');

    try {
      const response = await auth.login(email, password);
      if (response.data.success && response.data.data) {
        localStorage.setItem('webmail_token', response.data.data.token);
        localStorage.setItem('webmail_email', response.data.data.email);
        setToken(response.data.data.token);
        setEmail(response.data.data.email);
      } else {
        setLoginError(response.data.message || 'Giris basarisiz');
      }
    } catch (err: any) {
      setLoginError(err.response?.data?.message || 'Giris basarisiz');
    } finally {
      setLoading(false);
    }
  };

  const handleLogout = () => {
    localStorage.removeItem('webmail_token');
    localStorage.removeItem('webmail_email');
    setToken(null);
    setEmail('');
    setPassword('');
    setFolderList([]);
    setMessageList([]);
    setSelectedMessage(null);
  };

  const loadFolders = async () => {
    try {
      const response = await folders.list();
      if (response.data.success && response.data.data) {
        setFolderList(response.data.data);
      }
    } catch (err) {
      console.error('Failed to load folders', err);
    }
  };

  const loadMessages = async () => {
    setLoadingMessages(true);
    setSelectedMessage(null);
    try {
      const response = await messages.list(selectedFolder, page);
      if (response.data.success && response.data.data) {
        setMessageList(response.data.data.messages || []);
        setTotalPages(response.data.data.pages);
      }
    } catch (err) {
      console.error('Failed to load messages', err);
    } finally {
      setLoadingMessages(false);
    }
  };

  const openMessage = async (uid: number) => {
    try {
      const response = await messages.get(selectedFolder, uid);
      if (response.data.success && response.data.data) {
        setSelectedMessage(response.data.data);
        // Update message list to show as read
        setMessageList(prev => prev.map(m => m.uid === uid ? { ...m, seen: true } : m));
        // Update folder unread count
        setFolderList(prev => prev.map(f =>
          f.name === selectedFolder ? { ...f, unread: Math.max(0, f.unread - 1) } : f
        ));
      }
    } catch (err) {
      console.error('Failed to load message', err);
    }
  };

  const deleteMessage = async (uid: number) => {
    if (!confirm('Bu mesaji silmek istediginize emin misiniz?')) return;

    try {
      await messages.delete(selectedFolder, uid);
      setSelectedMessage(null);
      loadMessages();
      loadFolders();
    } catch (err) {
      console.error('Failed to delete message', err);
    }
  };

  const toggleFlag = async (uid: number, flagged: boolean) => {
    try {
      await messages.updateFlags(selectedFolder, uid, { flagged: !flagged });
      setMessageList(prev => prev.map(m => m.uid === uid ? { ...m, flagged: !flagged } : m));
      if (selectedMessage?.uid === uid) {
        setSelectedMessage(prev => prev ? { ...prev, flagged: !flagged } : null);
      }
    } catch (err) {
      console.error('Failed to toggle flag', err);
    }
  };

  // Parse email address to extract name and email
  const parseEmailAddress = (addr: string): { name: string; email: string } => {
    // Format: "Name <email@domain.com>" or just "email@domain.com"
    const match = addr.match(/^(.+?)\s*<(.+?)>$/);
    if (match) {
      return { name: match[1].trim().replace(/^["']|["']$/g, ''), email: match[2].trim() };
    }
    // Just email address
    const emailOnly = addr.trim();
    const namePart = emailOnly.split('@')[0].replace(/[._-]/g, ' ');
    return { name: namePart.charAt(0).toUpperCase() + namePart.slice(1), email: emailOnly };
  };

  // Add contact from email address
  const addContactFromEmail = (addr: string) => {
    const parsed = parseEmailAddress(addr);
    setEditingContact({
      full_name: parsed.name,
      email: parsed.email,
      phone: ''
    });
    setShowContactModal(true);
  };

  // Get all email addresses from a message
  const getEmailAddressesFromMessage = (msg: MessageDetail): { label: string; addr: string }[] => {
    const addresses: { label: string; addr: string }[] = [];

    if (msg.from) {
      addresses.push({ label: 'Gonderen', addr: msg.from });
    }

    if (msg.to) {
      msg.to.split(',').forEach(addr => {
        const trimmed = addr.trim();
        if (trimmed && !trimmed.includes(email)) { // Don't add self
          addresses.push({ label: 'Alici', addr: trimmed });
        }
      });
    }

    if (msg.cc) {
      msg.cc.split(',').forEach(addr => {
        const trimmed = addr.trim();
        if (trimmed && !trimmed.includes(email)) {
          addresses.push({ label: 'CC', addr: trimmed });
        }
      });
    }

    return addresses;
  };

  const replyTo = (msg: MessageDetail) => {
    setComposeTo(msg.from.includes('<') ? msg.from.match(/<(.+)>/)?.[1] || msg.from : msg.from);
    setComposeSubject(msg.subject.startsWith('Re:') ? msg.subject : `Re: ${msg.subject}`);
    setComposeBody(`\n\n--- Orijinal Mesaj ---\nKimden: ${msg.from}\nTarih: ${new Date(msg.date).toLocaleString('tr-TR')}\n\n${msg.text_body || ''}`);

    // Set attachments for reply
    if (msg.attachments && msg.attachments.length > 0) {
      setReplyAttachments(msg.attachments.map(att => ({
        index: att.index,
        filename: att.filename,
        selected: false
      })));
      setReplySource({ folder: selectedFolder, uid: msg.uid });
    } else {
      setReplyAttachments([]);
      setReplySource(null);
    }

    setShowCompose(true);
  };

  const handleSend = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!composeTo.trim()) return;

    setSending(true);
    try {
      const toAddrs = composeTo.split(/[,;]/).map(s => s.trim()).filter(Boolean);
      const ccAddrs = composeCc ? composeCc.split(/[,;]/).map(s => s.trim()).filter(Boolean) : undefined;

      // Get selected attachments to forward
      const selectedAtts = replyAttachments.filter(a => a.selected);
      const forwardAttachments = selectedAtts.length > 0 && replySource ? {
        source_folder: replySource.folder,
        source_uid: replySource.uid,
        indexes: selectedAtts.map(a => a.index)
      } : undefined;

      await messages.send({
        to: toAddrs,
        cc: ccAddrs,
        subject: composeSubject,
        body: composeBody,
        is_html: false,
        forward_attachments: forwardAttachments,
      });

      setShowCompose(false);
      setComposeTo('');
      setComposeCc('');
      setComposeSubject('');
      setComposeBody('');
      setReplyAttachments([]);
      setReplySource(null);
      alert('Mesaj gonderildi');
    } catch (err: any) {
      alert(err.response?.data?.message || 'Gonderme basarisiz');
    } finally {
      setSending(false);
    }
  };

  const formatDate = (dateStr: string) => {
    const date = new Date(dateStr);
    const now = new Date();
    const isToday = date.toDateString() === now.toDateString();

    if (isToday) {
      return date.toLocaleTimeString('tr-TR', { hour: '2-digit', minute: '2-digit' });
    }
    return date.toLocaleDateString('tr-TR', { day: '2-digit', month: '2-digit' });
  };

  const formatSize = (bytes: number) => {
    if (bytes < 1024) return bytes + ' B';
    if (bytes < 1024 * 1024) return (bytes / 1024).toFixed(1) + ' KB';
    return (bytes / (1024 * 1024)).toFixed(1) + ' MB';
  };

  const getFolderIcon = (name: string) => {
    const lower = name.toLowerCase();
    if (lower === 'inbox') return '📥';
    if (lower.includes('sent')) return '📤';
    if (lower.includes('draft')) return '📝';
    if (lower.includes('trash') || lower.includes('deleted')) return '🗑️';
    if (lower.includes('spam') || lower.includes('junk')) return '⚠️';
    if (lower.includes('archive')) return '📦';
    return '📁';
  };

  const getFolderDisplayName = (name: string) => {
    const lower = name.toLowerCase();
    if (lower === 'inbox') return 'Gelen Kutusu';
    if (lower.includes('sent')) return 'Gonderilenler';
    if (lower.includes('draft')) return 'Taslaklar';
    if (lower.includes('trash') || lower.includes('deleted')) return 'Cop Kutusu';
    if (lower.includes('spam') || lower.includes('junk')) return 'Spam';
    if (lower.includes('archive')) return 'Arsiv';
    return name.replace('INBOX.', '');
  };

  // Calendar functions
  const loadEvents = async () => {
    try {
      const response = await events.list();
      if (response.data.success && response.data.data) {
        setEventList(response.data.data);
      }
    } catch (err) {
      console.error('Failed to load events', err);
    }
  };

  const saveEvent = async (event: CalendarEvent) => {
    try {
      if (event.id) {
        await events.update(event.id, event);
      } else {
        await events.create(event);
      }
      loadEvents();
      setShowEventModal(false);
      setEditingEvent(null);
    } catch (err) {
      console.error('Failed to save event', err);
    }
  };

  const deleteEvent = async (id: number) => {
    if (!confirm('Bu etkinligi silmek istediginize emin misiniz?')) return;
    try {
      await events.delete(id);
      loadEvents();
    } catch (err) {
      console.error('Failed to delete event', err);
    }
  };

  const getEventsForDate = (date: Date) => {
    return eventList.filter(e => {
      const eventDate = new Date(e.start_time);
      return eventDate.toDateString() === date.toDateString();
    });
  };

  const getDaysInMonth = (date: Date) => {
    const year = date.getFullYear();
    const month = date.getMonth();
    const firstDay = new Date(year, month, 1);
    const lastDay = new Date(year, month + 1, 0);
    const days: Date[] = [];

    // Add empty days for alignment
    for (let i = 0; i < firstDay.getDay(); i++) {
      days.push(new Date(year, month, -i));
    }
    days.reverse();

    // Add days of the month
    for (let i = 1; i <= lastDay.getDate(); i++) {
      days.push(new Date(year, month, i));
    }

    return days;
  };

  // Shared Calendar functions
  const loadSharedCalendars = async () => {
    try {
      const response = await sharedCalendars.list();
      if (response.data.success && response.data.data) {
        setSharedCalendarList(response.data.data);
      }
    } catch (err) {
      console.error('Failed to load shared calendars', err);
    }
  };

  const loadSharedEvents = async (calId: number) => {
    try {
      const response = await sharedCalendars.getEvents(calId);
      if (response.data.success && response.data.data) {
        setSharedEventList(response.data.data);
      }
    } catch (err) {
      console.error('Failed to load shared events', err);
    }
  };

  const loadCalendarMembers = async (calId: number) => {
    try {
      const response = await sharedCalendars.getMembers(calId);
      if (response.data.success && response.data.data) {
        setCalendarMembers(response.data.data);
      }
    } catch (err) {
      console.error('Failed to load members', err);
    }
  };

  const loadDomainUsers = async () => {
    try {
      const response = await domainUsers.list();
      if (response.data.success && response.data.data) {
        setDomainUserList(response.data.data);
      }
    } catch (err) {
      console.error('Failed to load domain users', err);
    }
  };

  const saveSharedCalendar = async (cal: Partial<SharedCalendar>) => {
    try {
      if (cal.id) {
        await sharedCalendars.update(cal.id, cal);
      } else {
        await sharedCalendars.create(cal);
      }
      loadSharedCalendars();
      setShowSharedCalendarModal(false);
      setEditingSharedCalendar(null);
    } catch (err) {
      console.error('Failed to save shared calendar', err);
    }
  };

  const deleteSharedCalendar = async (id: number) => {
    if (!confirm('Bu grup takvimini silmek istediginize emin misiniz?')) return;
    try {
      await sharedCalendars.delete(id);
      setSelectedSharedCalendar(null);
      loadSharedCalendars();
    } catch (err) {
      console.error('Failed to delete shared calendar', err);
    }
  };

  const addMember = async () => {
    if (!selectedSharedCalendar || !newMemberEmail) return;
    try {
      await sharedCalendars.addMember(selectedSharedCalendar.id, newMemberEmail, newMemberRole);
      loadCalendarMembers(selectedSharedCalendar.id);
      setNewMemberEmail('');
      setNewMemberRole('viewer');
    } catch (err) {
      console.error('Failed to add member', err);
    }
  };

  const removeMember = async (memberId: number) => {
    if (!selectedSharedCalendar) return;
    if (!confirm('Bu uyeyi kaldirmak istediginize emin misiniz?')) return;
    try {
      await sharedCalendars.removeMember(selectedSharedCalendar.id, memberId);
      loadCalendarMembers(selectedSharedCalendar.id);
    } catch (err) {
      console.error('Failed to remove member', err);
    }
  };

  const saveSharedEvent = async (event: SharedEvent) => {
    if (!selectedSharedCalendar) return;
    try {
      if (event.id) {
        await sharedCalendars.updateEvent(selectedSharedCalendar.id, event.id, event);
      } else {
        await sharedCalendars.createEvent(selectedSharedCalendar.id, event);
      }
      loadSharedEvents(selectedSharedCalendar.id);
      setShowSharedEventModal(false);
      setEditingSharedEvent(null);
    } catch (err) {
      console.error('Failed to save shared event', err);
    }
  };

  const deleteSharedEvent = async (eventId: number) => {
    if (!selectedSharedCalendar) return;
    if (!confirm('Bu etkinligi silmek istediginize emin misiniz?')) return;
    try {
      await sharedCalendars.deleteEvent(selectedSharedCalendar.id, eventId);
      loadSharedEvents(selectedSharedCalendar.id);
    } catch (err) {
      console.error('Failed to delete shared event', err);
    }
  };

  const loadFreeBusy = async () => {
    if (!selectedSharedCalendar) return;
    try {
      const start = selectedDate.toISOString().slice(0, 10);
      const end = new Date(selectedDate.getTime() + 7 * 24 * 60 * 60 * 1000).toISOString().slice(0, 10);
      const response = await sharedCalendars.getFreeBusy(selectedSharedCalendar.id, start, end);
      if (response.data.success && response.data.data) {
        setFreeBusyData(response.data.data);
        setShowFreeBusyModal(true);
      }
    } catch (err) {
      console.error('Failed to load free/busy', err);
    }
  };

  const findAvailableTime = async () => {
    if (!selectedSharedCalendar) return;
    try {
      const response = await sharedCalendars.findAvailableTime(selectedSharedCalendar.id, findTimeParams);
      if (response.data.success && response.data.data) {
        setAvailableSlots(response.data.data);
      }
    } catch (err) {
      console.error('Failed to find available time', err);
    }
  };

  const getSharedEventsForDate = (date: Date) => {
    return sharedEventList.filter(e => {
      const eventDate = new Date(e.start_time);
      return eventDate.toDateString() === date.toDateString();
    });
  };

  useEffect(() => {
    if (token && activeTab === 'calendar' && calendarView === 'group') {
      loadSharedCalendars();
      loadDomainUsers();
    }
  }, [token, activeTab, calendarView]);

  useEffect(() => {
    if (selectedSharedCalendar) {
      loadSharedEvents(selectedSharedCalendar.id);
      loadCalendarMembers(selectedSharedCalendar.id);
    }
  }, [selectedSharedCalendar]);

  // Contacts functions
  const loadContacts = async () => {
    try {
      const response = await contacts.list(contactSearch || undefined);
      if (response.data.success && response.data.data) {
        setContactList(response.data.data);
      }
    } catch (err) {
      console.error('Failed to load contacts', err);
    }
  };

  const saveContact = async (contact: Contact) => {
    try {
      if (contact.id) {
        await contacts.update(contact.id, contact);
      } else {
        await contacts.create(contact);
      }
      loadContacts();
      setShowContactModal(false);
      setEditingContact(null);
    } catch (err) {
      console.error('Failed to save contact', err);
    }
  };

  const deleteContact = async (id: number) => {
    if (!confirm('Bu kisiyi silmek istediginize emin misiniz?')) return;
    try {
      await contacts.delete(id);
      loadContacts();
    } catch (err) {
      console.error('Failed to delete contact', err);
    }
  };

  useEffect(() => {
    if (token && activeTab === 'calendar') {
      loadEvents();
    }
  }, [token, activeTab]);

  useEffect(() => {
    if (token && activeTab === 'contacts') {
      loadContacts();
    }
  }, [token, activeTab, contactSearch]);

  // Login screen
  if (!token) {
    return (
      <div className="login-container">
        <form className="login-box" onSubmit={handleLogin}>
          <h1>Webmail</h1>
          {loginError && <div className="error">{loginError}</div>}
          <div className="form-group">
            <label>E-posta</label>
            <input
              type="email"
              value={email}
              onChange={(e) => setEmail(e.target.value)}
              placeholder="kullanici@domain.com"
              required
              autoFocus
            />
          </div>
          <div className="form-group">
            <label>Sifre</label>
            <input
              type="password"
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              required
            />
          </div>
          <button type="submit" disabled={loading}>
            {loading ? 'Giris yapiliyor...' : 'Giris Yap'}
          </button>
        </form>
      </div>
    );
  }

  return (
    <div className="webmail">
      <header className="header">
        <div className="logo">Webmail</div>
        <nav className="main-tabs">
          <button className={activeTab === 'mail' ? 'active' : ''} onClick={() => setActiveTab('mail')}>
            📧 E-posta
          </button>
          <button className={activeTab === 'calendar' ? 'active' : ''} onClick={() => setActiveTab('calendar')}>
            📅 Takvim
          </button>
          <button className={activeTab === 'contacts' ? 'active' : ''} onClick={() => setActiveTab('contacts')}>
            👥 Rehber
          </button>
        </nav>
        <div className="user-info">
          <span>{email}</span>
          <button onClick={handleLogout}>Cikis</button>
        </div>
      </header>

      <div className="main">
        {activeTab === 'mail' && (
        <aside className="sidebar">
          <button className="compose-btn" onClick={() => setShowCompose(true)}>
            + Yeni Mesaj
          </button>
          <nav className="folder-list">
            {folderList.map((folder) => (
              <div
                key={folder.name}
                className={`folder-item ${selectedFolder === folder.name ? 'active' : ''}`}
                onClick={() => { setSelectedFolder(folder.name); setPage(1); }}
              >
                <span className="folder-icon">{getFolderIcon(folder.name)}</span>
                <span className="folder-name">{getFolderDisplayName(folder.name)}</span>
                {folder.unread > 0 && <span className="unread-badge">{folder.unread}</span>}
              </div>
            ))}
          </nav>
        </aside>
        )}

        <main className="content">
          {activeTab === 'mail' && (
          <>
          {selectedMessage ? (
            <div className="message-view">
              <div className="message-view-header">
                <button onClick={() => setSelectedMessage(null)}>← Geri</button>
                <div className="message-actions">
                  <button onClick={() => replyTo(selectedMessage)}>Yanitla</button>
                  <button onClick={() => toggleFlag(selectedMessage.uid, selectedMessage.flagged)}>
                    {selectedMessage.flagged ? '★' : '☆'}
                  </button>
                  <button onClick={() => deleteMessage(selectedMessage.uid)}>Sil</button>
                </div>
              </div>
              <div className="message-view-content">
                <h2>{selectedMessage.subject || '(Konusuz)'}</h2>
                <div className="message-meta">
                  <div><strong>Kimden:</strong> {selectedMessage.from}</div>
                  <div><strong>Kime:</strong> {selectedMessage.to}</div>
                  {selectedMessage.cc && <div><strong>CC:</strong> {selectedMessage.cc}</div>}
                  <div><strong>Tarih:</strong> {new Date(selectedMessage.date).toLocaleString('tr-TR')}</div>
                </div>
                <div className="add-contact-from-email">
                  <span className="add-contact-label">Rehbere Ekle:</span>
                  {getEmailAddressesFromMessage(selectedMessage).map((item, idx) => (
                    <button
                      key={idx}
                      className="add-contact-btn"
                      onClick={() => addContactFromEmail(item.addr)}
                      title={`${item.label}: ${item.addr}`}
                    >
                      + {parseEmailAddress(item.addr).name}
                    </button>
                  ))}
                </div>
                {selectedMessage.attachments && selectedMessage.attachments.length > 0 && (
                  <div className="attachments">
                    <strong>Ekler:</strong>
                    {selectedMessage.attachments.map((att) => (
                      <a
                        key={att.index}
                        href={getAttachmentUrl(selectedFolder, selectedMessage.uid, att.index)}
                        target="_blank"
                        rel="noopener noreferrer"
                        className="attachment-link"
                      >
                        📎 {att.filename} ({formatSize(att.size)})
                      </a>
                    ))}
                  </div>
                )}
                <div className="message-body">
                  {selectedMessage.html_body ? (
                    <iframe
                      srcDoc={selectedMessage.html_body}
                      sandbox="allow-same-origin"
                      title="Email content"
                    />
                  ) : (
                    <pre>{selectedMessage.text_body}</pre>
                  )}
                </div>
              </div>
            </div>
          ) : (
            <div className="message-list-container">
              <div className="message-list-header">
                <h2>{getFolderDisplayName(selectedFolder)}</h2>
                <button onClick={loadMessages} disabled={loadingMessages}>
                  {loadingMessages ? 'Yukleniyor...' : 'Yenile'}
                </button>
              </div>
              {loadingMessages ? (
                <div className="loading">Yukleniyor...</div>
              ) : messageList.length === 0 ? (
                <div className="empty">Bu klasorde mesaj yok</div>
              ) : (
                <>
                  <div className="message-list">
                    {messageList.map((msg) => (
                      <div
                        key={msg.uid}
                        className={`message-item ${!msg.seen ? 'unread' : ''}`}
                        onClick={() => openMessage(msg.uid)}
                      >
                        <div className="message-flag" onClick={(e) => { e.stopPropagation(); toggleFlag(msg.uid, msg.flagged); }}>
                          {msg.flagged ? '★' : '☆'}
                        </div>
                        <div className="message-from">{msg.from.split('<')[0].trim() || msg.from}</div>
                        <div className="message-subject">
                          {msg.has_attachment && '📎 '}
                          {msg.subject || '(Konusuz)'}
                        </div>
                        <div className="message-date">{formatDate(msg.date)}</div>
                      </div>
                    ))}
                  </div>
                  {totalPages > 1 && (
                    <div className="pagination">
                      <button disabled={page <= 1} onClick={() => setPage(p => p - 1)}>Onceki</button>
                      <span>{page} / {totalPages}</span>
                      <button disabled={page >= totalPages} onClick={() => setPage(p => p + 1)}>Sonraki</button>
                    </div>
                  )}
                </>
              )}
            </div>
          )}
          </>
          )}

          {activeTab === 'calendar' && (
            <div className="calendar-view">
              <div className="calendar-tabs">
                <button className={calendarView === 'personal' ? 'active' : ''} onClick={() => setCalendarView('personal')}>
                  Kisisel Takvim
                </button>
                <button className={calendarView === 'group' ? 'active' : ''} onClick={() => setCalendarView('group')}>
                  Grup Takvimleri
                </button>
              </div>

              {calendarView === 'personal' && (
                <>
                  <div className="calendar-header">
                    <button onClick={() => setSelectedDate(new Date(selectedDate.getFullYear(), selectedDate.getMonth() - 1))}>←</button>
                    <h2>{selectedDate.toLocaleDateString('tr-TR', { month: 'long', year: 'numeric' })}</h2>
                    <button onClick={() => setSelectedDate(new Date(selectedDate.getFullYear(), selectedDate.getMonth() + 1))}>→</button>
                    <button className="add-btn" onClick={() => { setEditingEvent(null); setShowEventModal(true); }}>+ Etkinlik Ekle</button>
                  </div>
                  <div className="calendar-grid">
                    <div className="calendar-weekdays">
                      {['Paz', 'Pzt', 'Sal', 'Car', 'Per', 'Cum', 'Cmt'].map(day => (
                        <div key={day} className="weekday">{day}</div>
                      ))}
                    </div>
                    <div className="calendar-days">
                      {getDaysInMonth(selectedDate).map((day, idx) => {
                        const isCurrentMonth = day.getMonth() === selectedDate.getMonth();
                        const isToday = day.toDateString() === new Date().toDateString();
                        const dayEvents = getEventsForDate(day);
                        return (
                          <div
                            key={idx}
                            className={`calendar-day ${!isCurrentMonth ? 'other-month' : ''} ${isToday ? 'today' : ''}`}
                            onClick={() => {
                              setEditingEvent({
                                summary: '',
                                start_time: day.toISOString().slice(0, 16),
                                end_time: new Date(day.getTime() + 3600000).toISOString().slice(0, 16),
                                all_day: false
                              });
                              setShowEventModal(true);
                            }}
                          >
                            <span className="day-number">{day.getDate()}</span>
                            {dayEvents.map(event => (
                              <div
                                key={event.id}
                                className="day-event"
                                onClick={(e) => {
                                  e.stopPropagation();
                                  setEditingEvent(event);
                                  setShowEventModal(true);
                                }}
                              >
                                {event.summary}
                              </div>
                            ))}
                          </div>
                        );
                      })}
                    </div>
                  </div>
                </>
              )}

              {calendarView === 'group' && (
                <div className="group-calendar-container">
                  <div className="group-calendar-sidebar">
                    <button className="add-btn" onClick={() => { setEditingSharedCalendar(null); setShowSharedCalendarModal(true); }}>
                      + Grup Takvimi Olustur
                    </button>
                    <div className="shared-calendar-list">
                      {sharedCalendarList.length === 0 ? (
                        <div className="empty-small">Grup takvimi yok</div>
                      ) : (
                        sharedCalendarList.map(cal => (
                          <div
                            key={cal.id}
                            className={`shared-calendar-item ${selectedSharedCalendar?.id === cal.id ? 'active' : ''}`}
                            onClick={() => setSelectedSharedCalendar(cal)}
                          >
                            <span className="cal-color" style={{ background: cal.color }}></span>
                            <span className="cal-name">{cal.name}</span>
                            <span className="cal-role">{cal.role === 'owner' ? '(Sahip)' : cal.role === 'editor' ? '(Duzenleyici)' : ''}</span>
                          </div>
                        ))
                      )}
                    </div>
                  </div>

                  {selectedSharedCalendar ? (
                    <div className="group-calendar-main">
                      <div className="group-calendar-header">
                        <h3 style={{ color: selectedSharedCalendar.color }}>{selectedSharedCalendar.name}</h3>
                        <div className="group-calendar-actions">
                          <button onClick={() => setShowMembersModal(true)}>Uyeler ({calendarMembers.length})</button>
                          <button onClick={loadFreeBusy}>Bos/Dolu</button>
                          <button onClick={() => setShowFindTimeModal(true)}>Bos Zaman Bul</button>
                          {(selectedSharedCalendar.role === 'owner' || selectedSharedCalendar.role === 'editor') && (
                            <button className="add-btn" onClick={() => {
                              setEditingSharedEvent({
                                summary: '',
                                start_time: new Date().toISOString().slice(0, 16),
                                end_time: new Date(Date.now() + 3600000).toISOString().slice(0, 16),
                                all_day: false
                              });
                              setShowSharedEventModal(true);
                            }}>+ Etkinlik</button>
                          )}
                          {selectedSharedCalendar.role === 'owner' && (
                            <>
                              <button onClick={() => { setEditingSharedCalendar(selectedSharedCalendar); setShowSharedCalendarModal(true); }}>Duzenle</button>
                              <button className="delete-btn" onClick={() => deleteSharedCalendar(selectedSharedCalendar.id)}>Sil</button>
                            </>
                          )}
                        </div>
                      </div>

                      <div className="calendar-header">
                        <button onClick={() => setSelectedDate(new Date(selectedDate.getFullYear(), selectedDate.getMonth() - 1))}>←</button>
                        <h2>{selectedDate.toLocaleDateString('tr-TR', { month: 'long', year: 'numeric' })}</h2>
                        <button onClick={() => setSelectedDate(new Date(selectedDate.getFullYear(), selectedDate.getMonth() + 1))}>→</button>
                      </div>

                      <div className="calendar-grid">
                        <div className="calendar-weekdays">
                          {['Paz', 'Pzt', 'Sal', 'Car', 'Per', 'Cum', 'Cmt'].map(day => (
                            <div key={day} className="weekday">{day}</div>
                          ))}
                        </div>
                        <div className="calendar-days">
                          {getDaysInMonth(selectedDate).map((day, idx) => {
                            const isCurrentMonth = day.getMonth() === selectedDate.getMonth();
                            const isToday = day.toDateString() === new Date().toDateString();
                            const dayEvents = getSharedEventsForDate(day);
                            return (
                              <div
                                key={idx}
                                className={`calendar-day ${!isCurrentMonth ? 'other-month' : ''} ${isToday ? 'today' : ''}`}
                                onClick={() => {
                                  if (selectedSharedCalendar.role === 'owner' || selectedSharedCalendar.role === 'editor') {
                                    setEditingSharedEvent({
                                      summary: '',
                                      start_time: day.toISOString().slice(0, 16),
                                      end_time: new Date(day.getTime() + 3600000).toISOString().slice(0, 16),
                                      all_day: false
                                    });
                                    setShowSharedEventModal(true);
                                  }
                                }}
                              >
                                <span className="day-number">{day.getDate()}</span>
                                {dayEvents.map(event => (
                                  <div
                                    key={event.id}
                                    className="day-event shared"
                                    style={{ background: selectedSharedCalendar.color }}
                                    onClick={(e) => {
                                      e.stopPropagation();
                                      setEditingSharedEvent(event);
                                      setShowSharedEventModal(true);
                                    }}
                                  >
                                    {event.summary}
                                  </div>
                                ))}
                              </div>
                            );
                          })}
                        </div>
                      </div>
                    </div>
                  ) : (
                    <div className="group-calendar-empty">
                      <p>Bir grup takvimi secin veya yeni bir tane olusturun</p>
                    </div>
                  )}
                </div>
              )}
            </div>
          )}

          {activeTab === 'contacts' && (
            <div className="contacts-view">
              <div className="contacts-header">
                <input
                  type="text"
                  placeholder="Kisi ara..."
                  value={contactSearch}
                  onChange={(e) => setContactSearch(e.target.value)}
                  className="contact-search"
                />
                <button className="add-btn" onClick={() => { setEditingContact(null); setShowContactModal(true); }}>+ Kisi Ekle</button>
              </div>
              <div className="contacts-list">
                {contactList.length === 0 ? (
                  <div className="empty">Kisi bulunamadi</div>
                ) : (
                  contactList.map(contact => (
                    <div key={contact.id} className="contact-item">
                      <div className="contact-avatar">{contact.full_name?.charAt(0).toUpperCase() || '?'}</div>
                      <div className="contact-info">
                        <div className="contact-name">{contact.full_name}</div>
                        {contact.email && <div className="contact-email">{contact.email}</div>}
                        {contact.phone && <div className="contact-phone">{contact.phone}</div>}
                      </div>
                      <div className="contact-actions">
                        {contact.email && (
                          <button onClick={() => {
                            setComposeTo(contact.email || '');
                            setActiveTab('mail');
                            setShowCompose(true);
                          }}>📧</button>
                        )}
                        <button onClick={() => { setEditingContact(contact); setShowContactModal(true); }}>✏️</button>
                        <button onClick={() => contact.id && deleteContact(contact.id)}>🗑️</button>
                      </div>
                    </div>
                  ))
                )}
              </div>
            </div>
          )}
        </main>
      </div>

      {showCompose && (
        <div className="modal-overlay" onClick={() => setShowCompose(false)}>
          <div className="compose-modal" onClick={(e) => e.stopPropagation()}>
            <div className="compose-header">
              <h2>Yeni Mesaj</h2>
              <button className="close-btn" onClick={() => setShowCompose(false)}>×</button>
            </div>
            <form onSubmit={handleSend}>
              <div className="compose-field">
                <label>Kime:</label>
                <input
                  type="text"
                  value={composeTo}
                  onChange={(e) => setComposeTo(e.target.value)}
                  placeholder="alici@domain.com"
                  required
                />
              </div>
              <div className="compose-field">
                <label>CC:</label>
                <input
                  type="text"
                  value={composeCc}
                  onChange={(e) => setComposeCc(e.target.value)}
                  placeholder="cc@domain.com"
                />
              </div>
              <div className="compose-field">
                <label>Konu:</label>
                <input
                  type="text"
                  value={composeSubject}
                  onChange={(e) => setComposeSubject(e.target.value)}
                  placeholder="Mesaj konusu"
                />
              </div>
              <div className="compose-body">
                <textarea
                  value={composeBody}
                  onChange={(e) => setComposeBody(e.target.value)}
                  placeholder="Mesajiniz..."
                />
              </div>
              {replyAttachments.length > 0 && (
                <div className="compose-attachments">
                  <label>Ekleri dahil et:</label>
                  <div className="attachment-list">
                    {replyAttachments.map((att, idx) => (
                      <label key={att.index} className="attachment-checkbox">
                        <input
                          type="checkbox"
                          checked={att.selected}
                          onChange={() => {
                            setReplyAttachments(prev => prev.map((a, i) =>
                              i === idx ? { ...a, selected: !a.selected } : a
                            ));
                          }}
                        />
                        <span>📎 {att.filename}</span>
                      </label>
                    ))}
                  </div>
                </div>
              )}
              <div className="compose-actions">
                <button type="submit" disabled={sending}>
                  {sending ? 'Gonderiliyor...' : 'Gonder'}
                </button>
                <button type="button" onClick={() => setShowCompose(false)}>Iptal</button>
              </div>
            </form>
          </div>
        </div>
      )}

      {showEventModal && (
        <div className="modal-overlay" onClick={() => setShowEventModal(false)}>
          <div className="event-modal" onClick={(e) => e.stopPropagation()}>
            <div className="modal-header">
              <h2>{editingEvent?.id ? 'Etkinlik Duzenle' : 'Yeni Etkinlik'}</h2>
              <button className="close-btn" onClick={() => setShowEventModal(false)}>×</button>
            </div>
            <form onSubmit={(e) => {
              e.preventDefault();
              if (editingEvent) saveEvent(editingEvent);
            }}>
              <div className="form-field">
                <label>Baslik:</label>
                <input
                  type="text"
                  value={editingEvent?.summary || ''}
                  onChange={(e) => setEditingEvent(prev => prev ? { ...prev, summary: e.target.value } : null)}
                  placeholder="Etkinlik basligi"
                  required
                />
              </div>
              <div className="form-field">
                <label>Baslangic:</label>
                <input
                  type="datetime-local"
                  value={editingEvent?.start_time?.slice(0, 16) || ''}
                  onChange={(e) => setEditingEvent(prev => prev ? { ...prev, start_time: e.target.value } : null)}
                  required
                />
              </div>
              <div className="form-field">
                <label>Bitis:</label>
                <input
                  type="datetime-local"
                  value={editingEvent?.end_time?.slice(0, 16) || ''}
                  onChange={(e) => setEditingEvent(prev => prev ? { ...prev, end_time: e.target.value } : null)}
                  required
                />
              </div>
              <div className="form-field">
                <label>Konum:</label>
                <input
                  type="text"
                  value={editingEvent?.location || ''}
                  onChange={(e) => setEditingEvent(prev => prev ? { ...prev, location: e.target.value } : null)}
                  placeholder="Konum"
                />
              </div>
              <div className="form-field">
                <label>Aciklama:</label>
                <textarea
                  value={editingEvent?.description || ''}
                  onChange={(e) => setEditingEvent(prev => prev ? { ...prev, description: e.target.value } : null)}
                  placeholder="Aciklama"
                />
              </div>
              <div className="form-actions">
                <button type="submit">Kaydet</button>
                {editingEvent?.id && (
                  <button type="button" className="delete-btn" onClick={() => {
                    if (editingEvent.id) deleteEvent(editingEvent.id);
                    setShowEventModal(false);
                  }}>Sil</button>
                )}
                <button type="button" onClick={() => setShowEventModal(false)}>Iptal</button>
              </div>
            </form>
          </div>
        </div>
      )}

      {showContactModal && (
        <div className="modal-overlay" onClick={() => setShowContactModal(false)}>
          <div className="contact-modal" onClick={(e) => e.stopPropagation()}>
            <div className="modal-header">
              <h2>{editingContact?.id ? 'Kisi Duzenle' : 'Yeni Kisi'}</h2>
              <button className="close-btn" onClick={() => setShowContactModal(false)}>×</button>
            </div>
            <form onSubmit={(e) => {
              e.preventDefault();
              if (editingContact) saveContact(editingContact);
            }}>
              <div className="form-field">
                <label>Ad Soyad:</label>
                <input
                  type="text"
                  value={editingContact?.full_name || ''}
                  onChange={(e) => setEditingContact(prev => prev ? { ...prev, full_name: e.target.value } : { full_name: e.target.value })}
                  placeholder="Ad Soyad"
                  required
                />
              </div>
              <div className="form-field">
                <label>E-posta:</label>
                <input
                  type="email"
                  value={editingContact?.email || ''}
                  onChange={(e) => setEditingContact(prev => prev ? { ...prev, email: e.target.value } : { full_name: '', email: e.target.value })}
                  placeholder="E-posta adresi"
                />
              </div>
              <div className="form-field">
                <label>Telefon:</label>
                <input
                  type="tel"
                  value={editingContact?.phone || ''}
                  onChange={(e) => setEditingContact(prev => prev ? { ...prev, phone: e.target.value } : { full_name: '', phone: e.target.value })}
                  placeholder="Telefon numarasi"
                />
              </div>
              <div className="form-actions">
                <button type="submit">Kaydet</button>
                {editingContact?.id && (
                  <button type="button" className="delete-btn" onClick={() => {
                    if (editingContact.id) deleteContact(editingContact.id);
                    setShowContactModal(false);
                  }}>Sil</button>
                )}
                <button type="button" onClick={() => setShowContactModal(false)}>Iptal</button>
              </div>
            </form>
          </div>
        </div>
      )}

      {/* Shared Calendar Modal */}
      {showSharedCalendarModal && (
        <div className="modal-overlay" onClick={() => setShowSharedCalendarModal(false)}>
          <div className="event-modal" onClick={(e) => e.stopPropagation()}>
            <div className="modal-header">
              <h2>{editingSharedCalendar?.id ? 'Grup Takvimini Duzenle' : 'Yeni Grup Takvimi'}</h2>
              <button className="close-btn" onClick={() => setShowSharedCalendarModal(false)}>×</button>
            </div>
            <form onSubmit={(e) => {
              e.preventDefault();
              saveSharedCalendar(editingSharedCalendar || { name: '', color: '#9b59b6' });
            }}>
              <div className="form-field">
                <label>Takvim Adi:</label>
                <input
                  type="text"
                  value={editingSharedCalendar?.name || ''}
                  onChange={(e) => setEditingSharedCalendar(prev => prev ? { ...prev, name: e.target.value } : { id: 0, name: e.target.value, color: '#9b59b6', owner_id: 0 })}
                  placeholder="Takvim adi"
                  required
                />
              </div>
              <div className="form-field">
                <label>Renk:</label>
                <input
                  type="color"
                  value={editingSharedCalendar?.color || '#9b59b6'}
                  onChange={(e) => setEditingSharedCalendar(prev => prev ? { ...prev, color: e.target.value } : { id: 0, name: '', color: e.target.value, owner_id: 0 })}
                />
              </div>
              <div className="form-field">
                <label>Aciklama:</label>
                <textarea
                  value={editingSharedCalendar?.description || ''}
                  onChange={(e) => setEditingSharedCalendar(prev => prev ? { ...prev, description: e.target.value } : { id: 0, name: '', color: '#9b59b6', owner_id: 0, description: e.target.value })}
                  placeholder="Aciklama"
                />
              </div>
              <div className="form-actions">
                <button type="submit">Kaydet</button>
                <button type="button" onClick={() => setShowSharedCalendarModal(false)}>Iptal</button>
              </div>
            </form>
          </div>
        </div>
      )}

      {/* Members Modal */}
      {showMembersModal && selectedSharedCalendar && (
        <div className="modal-overlay" onClick={() => setShowMembersModal(false)}>
          <div className="members-modal" onClick={(e) => e.stopPropagation()}>
            <div className="modal-header">
              <h2>Takvim Uyeleri</h2>
              <button className="close-btn" onClick={() => setShowMembersModal(false)}>×</button>
            </div>
            <div className="members-content">
              <div className="owner-info">
                <strong>Sahip:</strong> {selectedSharedCalendar.owner_email}
              </div>
              <div className="members-list">
                {calendarMembers.length === 0 ? (
                  <div className="empty-small">Henuz uye yok</div>
                ) : (
                  calendarMembers.map(member => (
                    <div key={member.id} className="member-item">
                      <span className="member-email">{member.email}</span>
                      <span className="member-role">{member.role === 'editor' ? 'Duzenleyici' : 'Izleyici'}</span>
                      {selectedSharedCalendar.role === 'owner' && (
                        <button className="remove-btn" onClick={() => removeMember(member.id)}>Kaldir</button>
                      )}
                    </div>
                  ))
                )}
              </div>
              {selectedSharedCalendar.role === 'owner' && (
                <div className="add-member-form">
                  <h4>Uye Ekle</h4>
                  <select
                    value={newMemberEmail}
                    onChange={(e) => setNewMemberEmail(e.target.value)}
                  >
                    <option value="">Kullanici secin...</option>
                    {domainUserList.filter(u => u.email !== email && !calendarMembers.some(m => m.email === u.email)).map(user => (
                      <option key={user.id} value={user.email}>{user.email}</option>
                    ))}
                  </select>
                  <select value={newMemberRole} onChange={(e) => setNewMemberRole(e.target.value)}>
                    <option value="viewer">Izleyici</option>
                    <option value="editor">Duzenleyici</option>
                  </select>
                  <button onClick={addMember} disabled={!newMemberEmail}>Ekle</button>
                </div>
              )}
            </div>
          </div>
        </div>
      )}

      {/* Free/Busy Modal */}
      {showFreeBusyModal && (
        <div className="modal-overlay" onClick={() => setShowFreeBusyModal(false)}>
          <div className="freebusy-modal" onClick={(e) => e.stopPropagation()}>
            <div className="modal-header">
              <h2>Bos/Dolu Durumu (Gelecek 7 Gun)</h2>
              <button className="close-btn" onClick={() => setShowFreeBusyModal(false)}>×</button>
            </div>
            <div className="freebusy-content">
              {freeBusyData.length === 0 ? (
                <div className="empty-small">Veri bulunamadi</div>
              ) : (
                freeBusyData.map(user => (
                  <div key={user.user_id} className="freebusy-user">
                    <div className="freebusy-user-header">{user.email}</div>
                    {user.slots.length === 0 ? (
                      <div className="freebusy-free">Tum zamanlar bos</div>
                    ) : (
                      <div className="freebusy-slots">
                        {user.slots.map((slot, idx) => (
                          <div key={idx} className="freebusy-slot busy">
                            {new Date(slot.start_time).toLocaleString('tr-TR', { dateStyle: 'short', timeStyle: 'short' })} -
                            {new Date(slot.end_time).toLocaleTimeString('tr-TR', { hour: '2-digit', minute: '2-digit' })}
                          </div>
                        ))}
                      </div>
                    )}
                  </div>
                ))
              )}
            </div>
          </div>
        </div>
      )}

      {/* Find Time Modal */}
      {showFindTimeModal && (
        <div className="modal-overlay" onClick={() => setShowFindTimeModal(false)}>
          <div className="findtime-modal" onClick={(e) => e.stopPropagation()}>
            <div className="modal-header">
              <h2>Bos Zaman Bul</h2>
              <button className="close-btn" onClick={() => setShowFindTimeModal(false)}>×</button>
            </div>
            <div className="findtime-content">
              <div className="findtime-params">
                <div className="form-field">
                  <label>Baslangic Tarihi:</label>
                  <input
                    type="date"
                    value={findTimeParams.start_date}
                    onChange={(e) => setFindTimeParams(prev => ({ ...prev, start_date: e.target.value }))}
                  />
                </div>
                <div className="form-field">
                  <label>Bitis Tarihi:</label>
                  <input
                    type="date"
                    value={findTimeParams.end_date}
                    onChange={(e) => setFindTimeParams(prev => ({ ...prev, end_date: e.target.value }))}
                  />
                </div>
                <div className="form-field">
                  <label>Sure (dakika):</label>
                  <input
                    type="number"
                    value={findTimeParams.duration_mins}
                    onChange={(e) => setFindTimeParams(prev => ({ ...prev, duration_mins: parseInt(e.target.value) || 60 }))}
                    min="15"
                    step="15"
                  />
                </div>
                <div className="form-field inline">
                  <label>Calisma Saatleri:</label>
                  <input
                    type="number"
                    value={findTimeParams.workday_start}
                    onChange={(e) => setFindTimeParams(prev => ({ ...prev, workday_start: parseInt(e.target.value) || 9 }))}
                    min="0"
                    max="23"
                  />
                  <span>-</span>
                  <input
                    type="number"
                    value={findTimeParams.workday_end}
                    onChange={(e) => setFindTimeParams(prev => ({ ...prev, workday_end: parseInt(e.target.value) || 18 }))}
                    min="0"
                    max="23"
                  />
                </div>
                <button className="add-btn" onClick={findAvailableTime}>Bos Zaman Bul</button>
              </div>
              <div className="findtime-results">
                <h4>Uygun Zamanlar</h4>
                {availableSlots.length === 0 ? (
                  <div className="empty-small">Arama yapin veya uygun zaman bulunamadi</div>
                ) : (
                  <div className="available-slots">
                    {availableSlots.map((slot, idx) => (
                      <div
                        key={idx}
                        className="available-slot"
                        onClick={() => {
                          setEditingSharedEvent({
                            summary: '',
                            start_time: new Date(slot.start_time).toISOString().slice(0, 16),
                            end_time: new Date(slot.end_time).toISOString().slice(0, 16),
                            all_day: false
                          });
                          setShowFindTimeModal(false);
                          setShowSharedEventModal(true);
                        }}
                      >
                        {new Date(slot.start_time).toLocaleString('tr-TR', { weekday: 'short', month: 'short', day: 'numeric', hour: '2-digit', minute: '2-digit' })} -
                        {new Date(slot.end_time).toLocaleTimeString('tr-TR', { hour: '2-digit', minute: '2-digit' })}
                      </div>
                    ))}
                  </div>
                )}
              </div>
            </div>
          </div>
        </div>
      )}

      {/* Shared Event Modal */}
      {showSharedEventModal && (
        <div className="modal-overlay" onClick={() => setShowSharedEventModal(false)}>
          <div className="event-modal" onClick={(e) => e.stopPropagation()}>
            <div className="modal-header">
              <h2>{editingSharedEvent?.id ? 'Etkinlik Duzenle' : 'Yeni Grup Etkinligi'}</h2>
              <button className="close-btn" onClick={() => setShowSharedEventModal(false)}>×</button>
            </div>
            <form onSubmit={(e) => {
              e.preventDefault();
              if (editingSharedEvent) saveSharedEvent(editingSharedEvent);
            }}>
              <div className="form-field">
                <label>Baslik:</label>
                <input
                  type="text"
                  value={editingSharedEvent?.summary || ''}
                  onChange={(e) => setEditingSharedEvent(prev => prev ? { ...prev, summary: e.target.value } : null)}
                  placeholder="Etkinlik basligi"
                  required
                />
              </div>
              <div className="form-field">
                <label>Baslangic:</label>
                <input
                  type="datetime-local"
                  value={editingSharedEvent?.start_time?.slice(0, 16) || ''}
                  onChange={(e) => setEditingSharedEvent(prev => prev ? { ...prev, start_time: e.target.value } : null)}
                  required
                />
              </div>
              <div className="form-field">
                <label>Bitis:</label>
                <input
                  type="datetime-local"
                  value={editingSharedEvent?.end_time?.slice(0, 16) || ''}
                  onChange={(e) => setEditingSharedEvent(prev => prev ? { ...prev, end_time: e.target.value } : null)}
                  required
                />
              </div>
              <div className="form-field">
                <label>Konum:</label>
                <input
                  type="text"
                  value={editingSharedEvent?.location || ''}
                  onChange={(e) => setEditingSharedEvent(prev => prev ? { ...prev, location: e.target.value } : null)}
                  placeholder="Konum"
                />
              </div>
              <div className="form-field">
                <label>Aciklama:</label>
                <textarea
                  value={editingSharedEvent?.description || ''}
                  onChange={(e) => setEditingSharedEvent(prev => prev ? { ...prev, description: e.target.value } : null)}
                  placeholder="Aciklama"
                />
              </div>
              <div className="form-actions">
                <button type="submit">Kaydet</button>
                {editingSharedEvent?.id && (
                  <button type="button" className="delete-btn" onClick={() => {
                    if (editingSharedEvent.id) deleteSharedEvent(editingSharedEvent.id);
                    setShowSharedEventModal(false);
                  }}>Sil</button>
                )}
                <button type="button" onClick={() => setShowSharedEventModal(false)}>Iptal</button>
              </div>
            </form>
          </div>
        </div>
      )}
    </div>
  );
}

export default App;
