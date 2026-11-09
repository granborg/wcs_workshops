import type { AuditType } from '@wcs/shared'
import { db } from './db.js'

export interface AuditInput {
  type: AuditType
  creatorId: string
  actorId?: string | null
  subjectId?: string | null
  ip?: string | null
  directoryId?: string | null
  videoId?: string | null
  qrCodeId?: string | null
  detail?: Record<string, unknown>
}

export async function audit(input: AuditInput): Promise<void> {
  await db.auditEvent.create({
    data: {
      type: input.type,
      creatorId: input.creatorId,
      actorId: input.actorId ?? null,
      subjectId: input.subjectId ?? null,
      ip: input.ip ?? null,
      directoryId: input.directoryId ?? null,
      videoId: input.videoId ?? null,
      qrCodeId: input.qrCodeId ?? null,
      detail: input.detail ? JSON.stringify(input.detail) : null,
    },
  })
}
