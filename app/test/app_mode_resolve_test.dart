import 'package:flutter_test/flutter_test.dart';
import 'package:shared_preferences/shared_preferences.dart';

import 'package:design_express/core/services/app_mode_service.dart';

void main() {
  TestWidgetsFlutterBinding.ensureInitialized();

  test('per-user local value wins', () async {
    SharedPreferences.setMockInitialValues({
      'app_mode_u1': 'market',
      'last_app_mode': 'learning',
    });
    expect(await AppModeService.resolveMode('u1', 'learning'), 'market');
  });

  test('device-wide fallback when per-user missing', () async {
    SharedPreferences.setMockInitialValues({'last_app_mode': 'market'});
    expect(await AppModeService.resolveMode('u1', 'learning'), 'market');
  });

  test('server mode used when nothing local', () async {
    SharedPreferences.setMockInitialValues({});
    expect(await AppModeService.resolveMode('u1', 'market'), 'market');
    // and it gets written back locally for next time
    expect(await AppModeService.getMode('u1'), 'market');
  });

  test('null (→ chooser) only when truly nothing is known', () async {
    SharedPreferences.setMockInitialValues({});
    expect(await AppModeService.resolveMode('u1', ''), isNull);
    expect(await AppModeService.resolveMode('u1', null), isNull);
  });

  test('setMode records both per-user and device-wide', () async {
    SharedPreferences.setMockInitialValues({});
    await AppModeService.setMode('u1', 'market');
    expect(await AppModeService.getMode('u1'), 'market');
    expect(await AppModeService.getLastMode(), 'market');
  });
}
