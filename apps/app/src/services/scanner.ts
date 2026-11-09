import { Capacitor } from '@capacitor/core'

export function scannerAvailable(): boolean {
  return Capacitor.isNativePlatform()
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
