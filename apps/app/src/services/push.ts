import { Capacitor } from '@capacitor/core'
import { api } from './api'

let registered = false

/** Ask for permission and register the device token with the API. Native platforms only. */
export async function registerPush(): Promise<void> {
  if (registered || !Capacitor.isNativePlatform()) return
  registered = true
  try {
    const { PushNotifications } = await import('@capacitor/push-notifications')
    const platform = Capacitor.getPlatform() === 'ios' ? 'ios' : 'android'

    await PushNotifications.addListener('registration', (token) => {
      void api.devices.register(token.value, platform).catch((err) => console.warn('[push] register failed', err))
    })
    await PushNotifications.addListener('registrationError', (err) => {
      console.warn('[push] registration error', err)
    })

    let perm = await PushNotifications.checkPermissions()
    if (perm.receive === 'prompt' || perm.receive === 'prompt-with-rationale') {
      perm = await PushNotifications.requestPermissions()
    }
    if (perm.receive === 'granted') await PushNotifications.register()
  } catch (err) {
    console.warn('[push] unavailable', err)
  }
}
