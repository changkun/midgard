// Copyright 2021 Changkun Ou. All rights reserved.
// Use of this source code is governed by a GPL-3.0
// license that can be found in the LICENSE file.

@import Quartz;
@import Foundation;
@import AVFoundation;
@import CoreMediaIO;

bool isScreenLocked() {
    @autoreleasepool {
        bool locked = false;
        CFDictionaryRef d = CGSessionCopyCurrentDictionary();
        id o = ((__bridge NSDictionary *) d)[@"CGSSessionScreenIsLocked"];
        if (o) {
            locked = [o boolValue];
        }
        CFRelease(d);
        return locked;
    }
}

OSStatus getVideoDeviceUID(CMIOObjectID device, NSString **uid) {
    @autoreleasepool {
        OSStatus err;
        UInt32 dataSize = 0;
        UInt32 dataUsed = 0;

        CMIOObjectPropertyAddress opa = {
            kCMIODevicePropertyDeviceUID,
            kCMIOObjectPropertyScopeWildcard,
            kCMIOObjectPropertyElementWildcard
        };

        err = CMIOObjectGetPropertyDataSize(device,
            &opa, 0, nil, &dataSize);
        if (err != kCMIOHardwareNoError) {
            return err;
        }

        CFStringRef uidStringRef = NULL;
        err = CMIOObjectGetPropertyData(device,
            &opa, 0, nil, dataSize, &dataUsed, &uidStringRef);
        if (err != kCMIOHardwareNoError) {
            return err;
        }

        *uid = (__bridge NSString *)uidStringRef;
        return err;
    }
}

bool isIgnoredDeviceUID(NSString *uid) {
    @autoreleasepool {
        // OBS virtual device always returns "is used" even when OBS is
        // not running
        if ([uid isEqual:@"obs-virtual-cam-device"]) {
            return true;
        }
        return false;
    }
}

OSStatus getVideoDeviceIsUsed(CMIOObjectID device, int *isUsed) {
    @autoreleasepool {
        OSStatus err;
        UInt32 dataSize = 0;
        UInt32 dataUsed = 0;

        CMIOObjectPropertyAddress prop = {
            kCMIODevicePropertyDeviceIsRunningSomewhere,
            kCMIOObjectPropertyScopeWildcard,
            kCMIOObjectPropertyElementWildcard
        };

        err = CMIOObjectGetPropertyDataSize(device,
            &prop, 0, nil, &dataSize);
        if (err != kCMIOHardwareNoError) {
            return err;
        }

        err = CMIOObjectGetPropertyData(device,
            &prop, 0, nil, dataSize, &dataUsed, isUsed);
        if (err != kCMIOHardwareNoError) {
            return err;
        }
        return err;
    }
}

// isCameraOn is a best effort camera on detection. It may report false positive.
bool isCameraOn() {
    @autoreleasepool {
        bool on = false;
        CMIOObjectPropertyAddress propertyAddress = {
            kCMIOHardwarePropertyDevices,
            kCMIOObjectPropertyScopeGlobal,
            kCMIOObjectPropertyElementMaster // This should be kCMIOObjectPropertyElementMain after macOS 12.0
        };
        UInt32 dataSize = 0;
        OSStatus status = CMIOObjectGetPropertyDataSize(
            kCMIOObjectSystemObject, &propertyAddress, 0, NULL, &dataSize);
        if(status != kCMIOHardwareNoError) {
            return false;
        }
        UInt32 deviceCount = (UInt32)(dataSize/sizeof(CMIOObjectID));
        CMIOObjectID *videoDevices = (CMIOObjectID *)(calloc(dataSize,1));
        if(NULL == videoDevices) {
            return false;
        }

        UInt32 used = 0;
        status = CMIOObjectGetPropertyData(kCMIOObjectSystemObject,
            &propertyAddress, 0, NULL, dataSize, &used, videoDevices);
        if(status != kCMIOHardwareNoError) {
            free(videoDevices);
            videoDevices = NULL;
            return false;
        }

        OSStatus err;
        int usedDevices = 0;
        int failedDeviceCount = 0;
        int ignoredDeviceCount = 0;
        for(UInt32 i = 0; i < deviceCount; ++i) {
            CMIOObjectID device = videoDevices[i];

            NSString *uid;
            err = getVideoDeviceUID(device, &uid);
            if (err) {
                failedDeviceCount++;
                continue;
            }

            if (isIgnoredDeviceUID(uid)) {
                ignoredDeviceCount++;
                continue;
            }

            CFStringRef deviceName = NULL;
            dataSize = sizeof(deviceName);
            propertyAddress.mSelector = kCMIOObjectPropertyName;
            status = CMIOObjectGetPropertyData(videoDevices[i],
                &propertyAddress, 0, NULL, dataSize, &used, &deviceName);
            if(status != kCMIOHardwareNoError) {
                continue;
            }


            int isDeviceUsed = 0;
            err = getVideoDeviceIsUsed(device, &isDeviceUsed);
            if (err) {
                failedDeviceCount++;
                continue;
            }

            if (isDeviceUsed != 0) {
                usedDevices++;
            }
        }
        if (failedDeviceCount == deviceCount) {
            free(videoDevices);
            videoDevices = NULL;
            return false;
        }
        if (usedDevices < 1) {
            free(videoDevices);
            videoDevices = NULL;
            return false;
        }

        free(videoDevices);
        videoDevices = NULL;
        return true;
    }
}

