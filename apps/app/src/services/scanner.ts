import { Capacitor } from '@capacitor/core'

export function isNativeScanner(): boolean {
  return Capacitor.isNativePlatform()
}

/**
 * True when the device can scan: the native app (ML Kit), or a browser with camera access.
 * Browsers expose the camera only in a secure context (https, or http://localhost).
 */
export function scannerAvailable(): boolean {
  return isNativeScanner() || (window.isSecureContext && !!navigator.mediaDevices?.getUserMedia)
}

/** Turns a getUserMedia failure into a message the user can act on. */
export function cameraErrorMessage(err: unknown): string {
  const name = (err as { name?: unknown })?.name
  if (name === 'NotAllowedError' || name === 'SecurityError') {
    return 'Camera access is blocked. Allow the camera for this site in your browser settings, or paste the link below.'
  }
  if (name === 'NotFoundError' || name === 'OverconstrainedError') return 'No camera was found on this device.'
  if (name === 'NotReadableError') return 'Another app is using the camera. Close it and try again.'
  return err instanceof Error ? err.message : String(err)
}

/** Opens the native scanner and resolves with the QR payload, or null when the user cancels. */
export async function scanQrCode(): Promise<string | null> {
  const { BarcodeScanner, BarcodeFormat } = await import('@capacitor-mlkit/barcode-scanning')

  const perm = await BarcodeScanner.requestPermissions()
  if (perm.camera !== 'granted' && perm.camera !== 'limited') {
    throw new Error('Camera permission was not granted')
  }

  if (Capacitor.getPlatform() === 'android') {
    const { available } = await BarcodeScanner.isGoogleBarcodeScannerModuleAvailable()
    if (!available) await BarcodeScanner.installGoogleBarcodeScannerModule()
  }

  const { barcodes } = await BarcodeScanner.scan({ formats: [BarcodeFormat.QrCode] })
  return barcodes[0]?.rawValue ?? null
}
