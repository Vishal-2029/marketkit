// ignore: avoid_web_libraries_in_flutter, deprecated_member_use
import 'dart:html' as html;

void showWebForegroundNotification(String title, String body) {
  if (html.Notification.supported) {
    html.Notification(title, body: body, icon: '/icons/Icon-192.png');
  }
}
