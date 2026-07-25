importScripts('https://www.gstatic.com/firebasejs/10.14.0/firebase-app-compat.js');
importScripts('https://www.gstatic.com/firebasejs/10.14.0/firebase-messaging-compat.js');

// Load FIREBASE_WEB_API_KEY and FIREBASE_WEB_APP_ID into the SW global scope.
// firebase-config.js is gitignored and generated at build/deploy time from
// environment variables. See web/firebase-config.js.template for the format.
try {
  importScripts('/firebase-config.js');
} catch (_) {
  // Running without firebase-config.js (local dev without the file, or first
  // deploy before it was generated). Background push will be silently disabled.
  console.warn('[FCM SW] /firebase-config.js not found — background notifications disabled. See web/firebase-config.js.template.');
}

// Every value comes from firebase-config.js — no hardcoded fallbacks, so this
// file never carries project-identifying values into version control.
const projectId = self.FIREBASE_WEB_PROJECT_ID;

const firebaseConfig = {
  apiKey: self.FIREBASE_WEB_API_KEY,
  authDomain: projectId ? `${projectId}.firebaseapp.com` : undefined,
  projectId: projectId,
  storageBucket: projectId ? `${projectId}.firebasestorage.app` : undefined,
  messagingSenderId: self.FIREBASE_WEB_SENDER_ID,
  appId: self.FIREBASE_WEB_APP_ID,
  measurementId: self.FIREBASE_WEB_MEASUREMENT_ID || undefined,
};

const isConfigured =
  Boolean(firebaseConfig.apiKey && firebaseConfig.appId && firebaseConfig.projectId);

if (isConfigured) {
  firebase.initializeApp(firebaseConfig);

  firebase.messaging().onBackgroundMessage((payload) => {
    const title = payload.notification?.title ?? payload.data?.title ?? 'Notification';
    const body = payload.notification?.body ?? payload.data?.body ?? '';
    self.registration.showNotification(title, {
      body,
      icon: '/icons/Icon-192.png',
      data: { screen: payload.data?.screen ?? '' },
    });
  });
} else {
  console.warn('[FCM SW] Firebase web config incomplete — background notifications disabled.');
}

self.addEventListener('notificationclick', (event) => {
  event.notification.close();
  const screen = event.notification.data?.screen ?? '';
  const pathMap = { gallery: '/gallery', community: '/community', home: '/home' };
  const path = pathMap[screen] ?? '/home';
  event.waitUntil(
    clients.matchAll({ type: 'window', includeUncontrolled: true }).then((windowClients) => {
      for (const client of windowClients) {
        if (client.url.includes(self.location.origin) && 'focus' in client) {
          client.navigate(path);
          return client.focus();
        }
      }
      return clients.openWindow(path);
    }),
  );
});
