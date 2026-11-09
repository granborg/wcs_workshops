import { db } from './db.js'

let messaging: import('firebase-admin/messaging').Messaging | null | undefined

async function getMessaging() {
  if (messaging !== undefined) return messaging
  try {
    const { getApps, initializeApp, applicationDefault } = await import('firebase-admin/app')
    const { getMessaging } = await import('firebase-admin/messaging')
    const app = getApps()[0] ?? initializeApp({ credential: applicationDefault() })
    messaging = getMessaging(app)
  } catch (err) {
    console.warn('[notify] Firebase Admin not configured; push disabled:', (err as Error).message)
    messaging = null
  }
  return messaging
}

/** Sends a push notification to every device registered to the user. Failures are logged, not thrown. */
export async function notifyUser(
  userId: string,
  title: string,
  body: string,
  data: Record<string, string> = {},
): Promise<void> {
  const devices = await db.device.findMany({ where: { userId } })
  if (devices.length === 0) return
  const m = await getMessaging()
  if (!m) {
    console.info(`[notify] (disabled) to user ${userId}: ${title} - ${body}`)
    return
  }
  const tokens = devices.map((d) => d.pushToken)
  try {
    const result = await m.sendEachForMulticast({ tokens, notification: { title, body }, data })
    const stale: string[] = []
    result.responses.forEach((r, i) => {
      const token = tokens[i]
      if (token && r.error?.code === 'messaging/registration-token-not-registered') stale.push(token)
    })
    if (stale.length > 0) {
      await db.device.deleteMany({ where: { pushToken: { in: stale } } })
    }
  } catch (err) {
    console.error('[notify] send failed:', (err as Error).message)
  }
}
