-- CreateTable
CREATE TABLE "User" (
    "id" TEXT NOT NULL PRIMARY KEY,
    "authUid" TEXT NOT NULL,
    "email" TEXT,
    "name" TEXT NOT NULL,
    "role" TEXT NOT NULL DEFAULT 'viewer',
    "bannerUrl" TEXT,
    "createdAt" DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "updatedAt" DATETIME NOT NULL
);

-- CreateTable
CREATE TABLE "Directory" (
    "id" TEXT NOT NULL PRIMARY KEY,
    "ownerId" TEXT NOT NULL,
    "parentId" TEXT,
    "name" TEXT NOT NULL,
    "createdAt" DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT "Directory_ownerId_fkey" FOREIGN KEY ("ownerId") REFERENCES "User" ("id") ON DELETE CASCADE ON UPDATE CASCADE,
    CONSTRAINT "Directory_parentId_fkey" FOREIGN KEY ("parentId") REFERENCES "Directory" ("id") ON DELETE CASCADE ON UPDATE CASCADE
);

-- CreateTable
CREATE TABLE "Video" (
    "id" TEXT NOT NULL PRIMARY KEY,
    "ownerId" TEXT NOT NULL,
    "directoryId" TEXT NOT NULL,
    "title" TEXT NOT NULL,
    "vimeoId" TEXT,
    "vimeoHash" TEXT,
    "status" TEXT NOT NULL DEFAULT 'uploading',
    "durationSec" INTEGER,
    "createdAt" DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "updatedAt" DATETIME NOT NULL,
    CONSTRAINT "Video_ownerId_fkey" FOREIGN KEY ("ownerId") REFERENCES "User" ("id") ON DELETE CASCADE ON UPDATE CASCADE,
    CONSTRAINT "Video_directoryId_fkey" FOREIGN KEY ("directoryId") REFERENCES "Directory" ("id") ON DELETE RESTRICT ON UPDATE CASCADE
);

-- CreateTable
CREATE TABLE "QrCode" (
    "id" TEXT NOT NULL PRIMARY KEY,
    "code" TEXT NOT NULL,
    "creatorId" TEXT NOT NULL,
    "directoryId" TEXT,
    "videoId" TEXT,
    "expiresAt" DATETIME,
    "revokedAt" DATETIME,
    "createdAt" DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT "QrCode_creatorId_fkey" FOREIGN KEY ("creatorId") REFERENCES "User" ("id") ON DELETE CASCADE ON UPDATE CASCADE,
    CONSTRAINT "QrCode_directoryId_fkey" FOREIGN KEY ("directoryId") REFERENCES "Directory" ("id") ON DELETE CASCADE ON UPDATE CASCADE,
    CONSTRAINT "QrCode_videoId_fkey" FOREIGN KEY ("videoId") REFERENCES "Video" ("id") ON DELETE CASCADE ON UPDATE CASCADE
);

-- CreateTable
CREATE TABLE "AccessGrant" (
    "id" TEXT NOT NULL PRIMARY KEY,
    "userId" TEXT NOT NULL,
    "directoryId" TEXT,
    "videoId" TEXT,
    "qrCodeId" TEXT,
    "createdAt" DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "revokedAt" DATETIME,
    CONSTRAINT "AccessGrant_userId_fkey" FOREIGN KEY ("userId") REFERENCES "User" ("id") ON DELETE CASCADE ON UPDATE CASCADE,
    CONSTRAINT "AccessGrant_directoryId_fkey" FOREIGN KEY ("directoryId") REFERENCES "Directory" ("id") ON DELETE CASCADE ON UPDATE CASCADE,
    CONSTRAINT "AccessGrant_videoId_fkey" FOREIGN KEY ("videoId") REFERENCES "Video" ("id") ON DELETE CASCADE ON UPDATE CASCADE,
    CONSTRAINT "AccessGrant_qrCodeId_fkey" FOREIGN KEY ("qrCodeId") REFERENCES "QrCode" ("id") ON DELETE SET NULL ON UPDATE CASCADE
);

-- CreateTable
CREATE TABLE "Device" (
    "id" TEXT NOT NULL PRIMARY KEY,
    "userId" TEXT NOT NULL,
    "pushToken" TEXT NOT NULL,
    "platform" TEXT NOT NULL,
    "createdAt" DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT "Device_userId_fkey" FOREIGN KEY ("userId") REFERENCES "User" ("id") ON DELETE CASCADE ON UPDATE CASCADE
);

-- CreateTable
CREATE TABLE "AuditEvent" (
    "id" TEXT NOT NULL PRIMARY KEY,
    "at" DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "type" TEXT NOT NULL,
    "creatorId" TEXT NOT NULL,
    "actorId" TEXT,
    "subjectId" TEXT,
    "ip" TEXT,
    "directoryId" TEXT,
    "videoId" TEXT,
    "qrCodeId" TEXT,
    "detail" TEXT
);

-- CreateIndex
CREATE UNIQUE INDEX "User_authUid_key" ON "User"("authUid");

-- CreateIndex
CREATE INDEX "Directory_ownerId_idx" ON "Directory"("ownerId");

-- CreateIndex
CREATE INDEX "Directory_parentId_idx" ON "Directory"("parentId");

-- CreateIndex
CREATE UNIQUE INDEX "Video_vimeoId_key" ON "Video"("vimeoId");

-- CreateIndex
CREATE INDEX "Video_directoryId_idx" ON "Video"("directoryId");

-- CreateIndex
CREATE INDEX "Video_ownerId_idx" ON "Video"("ownerId");

-- CreateIndex
CREATE UNIQUE INDEX "QrCode_code_key" ON "QrCode"("code");

-- CreateIndex
CREATE INDEX "QrCode_directoryId_idx" ON "QrCode"("directoryId");

-- CreateIndex
CREATE INDEX "QrCode_videoId_idx" ON "QrCode"("videoId");

-- CreateIndex
CREATE INDEX "AccessGrant_userId_idx" ON "AccessGrant"("userId");

-- CreateIndex
CREATE INDEX "AccessGrant_directoryId_idx" ON "AccessGrant"("directoryId");

-- CreateIndex
CREATE INDEX "AccessGrant_videoId_idx" ON "AccessGrant"("videoId");

-- CreateIndex
CREATE UNIQUE INDEX "Device_pushToken_key" ON "Device"("pushToken");

-- CreateIndex
CREATE INDEX "Device_userId_idx" ON "Device"("userId");

-- CreateIndex
CREATE INDEX "AuditEvent_creatorId_at_idx" ON "AuditEvent"("creatorId", "at");
