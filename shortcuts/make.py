#!/usr/bin/env python3
# Copyright 2026 Changkun Ou. All rights reserved.
# Use of this source code is governed by a GPL-3.0
# license that can be found in the LICENSE file.
"""Builds midgard's iPhone Shortcuts, signed so anyone can add them.

    python3 shortcuts/make.py        # on a Mac signed in to iCloud

writes "Get from Midgard" and "Send to Midgard" into api/rest/web/shortcuts,
which the server hands out from its web page. Each asks, when it is added,
for the server and for an app token, which the web page issues; they talk to
the server's API as any client does (docs/usage.md), and keep nothing.
"""

import os
import plistlib
import subprocess
import tempfile
import uuid

OUT = os.path.join(os.path.dirname(__file__), "..", "api", "rest", "web", "shortcuts")
OBJ = "￼"  # where a variable goes in a Shortcuts text


def new_id():
    return str(uuid.uuid4()).upper()


def text(s, **vars):
    """A text, with the named variables where OBJ is, in order."""
    attachments = {}
    at = 0
    for name in vars.get("order", []):
        at = s.index(OBJ, at)
        attachments["{%d, 1}" % at] = {"Type": "Variable", "VariableName": name}
        at += 1
    v = {"string": s}
    if attachments:
        v["attachmentsByRange"] = attachments
    return {"Value": v, "WFSerializationType": "WFTextTokenString"}


def var(name):
    return {"Value": {"Type": "Variable", "VariableName": name}, "WFSerializationType": "WFTextTokenAttachment"}


def as_text(ref):
    """ref, taken as text: an If compares a value whose type Shortcuts does
    not know, such as a dictionary's, as nothing, and shows the text it is
    compared with as missing."""
    v = dict(ref["Value"])
    v["Aggrandizements"] = [{"Type": "WFCoercionVariableAggrandizement", "CoercionItemClass": "WFStringContentItem"}]
    return {"Value": v, "WFSerializationType": ref["WFSerializationType"]}


def value_for(key, of, uid):
    """Get Dictionary Value, of the value for key: what it gets must be said,
    or Shortcuts asks to choose a value for each parameter."""
    return action("getvalueforkey", WFInput=of, WFGetDictionaryValueType="Value", WFDictionaryKey=key, UUID=uid)


def output(uid, name):
    return {"Value": {"Type": "ActionOutput", "OutputUUID": uid, "OutputName": name},
            "WFSerializationType": "WFTextTokenAttachment"}


def action(ident, **params):
    return {"WFWorkflowActionIdentifier": "is.workflow.actions." + ident, "WFWorkflowActionParameters": params}


def dictionary(items):
    """A dictionary field of text keys and text values."""
    return {"Value": {"WFDictionaryFieldValueItems": [
        {"WFItemType": 0, "WFKey": text(k), "WFValue": v} for k, v in items]},
        "WFSerializationType": "WFDictionaryFieldValue"}


def setup():
    """The actions every Shortcut begins with: the server and the token,
    which it asks for when it is added. Each Set Variable names the text it
    keeps: without an input, it keeps nothing, as Shortcuts passes no
    action's output on by itself."""
    server, token = new_id(), new_id()
    return [
        action("gettext", WFTextActionText="changkun.de", UUID=server),
        action("setvariable", WFVariableName="server", WFInput=output(server, "Text")),
        action("gettext", WFTextActionText="", UUID=token),
        action("setvariable", WFVariableName="token", WFInput=output(token, "Text")),
    ], [
        {"ActionIndex": 0, "Category": "Parameter", "ParameterKey": "WFTextActionText",
         "DefaultValue": "changkun.de", "Text": "Your Midgard server"},
        {"ActionIndex": 2, "Category": "Parameter", "ParameterKey": "WFTextActionText",
         "DefaultValue": "", "Text": "An app token for this device, from your Midgard web page"},
    ]


def request(method, json=None):
    """Get Contents of URL on the clipboard's endpoint, with the token."""
    uid = new_id()
    params = dict(
        UUID=uid, WFHTTPMethod=method, ShowHeaders=True,
        WFURL=text("https://" + OBJ + "/midgard/api/v1/clipboard", order=["server"]),
        WFHTTPHeaders=dictionary([("Authorization", text("Bearer " + OBJ, order=["token"]))]),
    )
    if json is not None:
        params.update(WFHTTPBodyType="JSON", WFJSONValues=dictionary(json))
    return uid, action("downloadurl", **params)


def said(uid, name="Dictionary Value"):
    """A text that is an earlier action's output."""
    return {"Value": {"string": OBJ, "attachmentsByRange": {"{0, 1}": {
        "Type": "ActionOutput", "OutputUUID": uid, "OutputName": name}}},
        "WFSerializationType": "WFTextTokenString"}


def get_shortcut():
    actions, questions = setup()
    resp, req = request("GET")
    actions.append(req)
    dict_uid, type_uid, data_uid, group = new_id(), new_id(), new_id(), new_id()
    decoded, cond = new_id(), new_id()
    actions += [
        action("detect.dictionary", WFInput=output(resp, "Contents of URL"), UUID=dict_uid),
        value_for("type", output(dict_uid, "Dictionary"), type_uid),
        action("setvariable", WFInput=output(type_uid, "Dictionary Value"), WFVariableName="kind"),
        value_for("data", output(dict_uid, "Dictionary"), data_uid),
        action("setvariable", WFInput=output(data_uid, "Dictionary Value"), WFVariableName="data"),
        # an answer without a type is the server saying why there is no copy
        action("conditional", GroupingIdentifier=group, WFControlFlowMode=0, WFCondition=101,
               WFInput={"Type": "Variable", "Variable": as_text(var("kind"))}),
        value_for("msg", output(dict_uid, "Dictionary"), cond),
        action("notification", WFNotificationActionTitle="Midgard", WFNotificationActionBody=said(cond)),
        action("exit"),
        action("conditional", GroupingIdentifier=group, WFControlFlowMode=2),
    ]
    image_group, named = new_id(), new_id()
    actions += [
        action("conditional", GroupingIdentifier=image_group, WFControlFlowMode=0, WFCondition=4,
               WFConditionalActionString="image/png", WFInput={"Type": "Variable", "Variable": as_text(var("kind"))}),
        action("base64encode", WFEncodeMode="Decode", WFInput=var("data"), UUID=decoded),
        # decoded bytes have no type: named .png, they are an image
        action("setitemname", WFInput=output(decoded, "Base64 Decoded"), WFName=text("clipboard.png"),
               WFDontIncludeFileExtension=False, UUID=named),
        action("setclipboard", WFLocalOnly=True, WFInput=output(named, "Renamed Item")),
        action("notification", WFNotificationActionTitle="Midgard",
               WFNotificationActionBody=text("An image from your devices is on the clipboard.")),
        action("conditional", GroupingIdentifier=image_group, WFControlFlowMode=1),
        action("setclipboard", WFLocalOnly=True, WFInput=var("data")),
        action("notification", WFNotificationActionTitle="Midgard",
               WFNotificationActionBody=text(OBJ, order=["data"])),
        action("conditional", GroupingIdentifier=image_group, WFControlFlowMode=2),
    ]
    return workflow(actions, questions, glyph=61540, color=946986751, share=False)


def send_shortcut():
    actions, questions = setup()
    clip, item_type, png, b64 = new_id(), new_id(), new_id(), new_id()
    has_input = new_id()
    actions += [
        # what is shared to it, or else the clipboard
        action("conditional", GroupingIdentifier=has_input, WFControlFlowMode=0, WFCondition=100,
               WFInput={"Type": "Variable", "Variable": {"Value": {"Type": "ExtensionInput"},
                                                         "WFSerializationType": "WFTextTokenAttachment"}}),
        action("setvariable", WFVariableName="content",
               WFInput={"Value": {"Type": "ExtensionInput"}, "WFSerializationType": "WFTextTokenAttachment"}),
        action("conditional", GroupingIdentifier=has_input, WFControlFlowMode=1),
        action("getclipboard", UUID=clip),
        action("setvariable", WFVariableName="content", WFInput=output(clip, "Clipboard")),
        action("conditional", GroupingIdentifier=has_input, WFControlFlowMode=2),
        action("getitemtype", WFInput=var("content"), UUID=item_type),
    ]
    image_group = new_id()
    resp_image, req_image = request("POST", [("type", text("image/png")), ("data", text(OBJ, order=["png"]))])
    resp_text, req_text = request("POST", [("type", text("text")), ("data", text(OBJ, order=["content"]))])
    actions += [
        action("conditional", GroupingIdentifier=image_group, WFControlFlowMode=0, WFCondition=99,
               WFConditionalActionString="Image",
               WFInput={"Type": "Variable", "Variable": as_text(output(item_type, "Type"))}),
        action("image.convert", WFImageFormat="PNG", WFImagePreserveMetadata=False, WFInput=var("content"), UUID=png),
        action("base64encode", WFEncodeMode="Encode", WFBase64LineBreakMode="None",
               WFInput=output(png, "Converted Image"), UUID=b64),
        action("setvariable", WFVariableName="png", WFInput=output(b64, "Base64 Encoded")),
        req_image,
        action("setvariable", WFVariableName="answer", WFInput=output(resp_image, "Contents of URL")),
        action("conditional", GroupingIdentifier=image_group, WFControlFlowMode=1),
        req_text,
        action("setvariable", WFVariableName="answer", WFInput=output(resp_text, "Contents of URL")),
        action("conditional", GroupingIdentifier=image_group, WFControlFlowMode=2),
    ]
    answer_dict, msg = new_id(), new_id()
    actions += [
        action("detect.dictionary", WFInput=var("answer"), UUID=answer_dict),
        value_for("msg", output(answer_dict, "Dictionary"), msg),
        action("notification", WFNotificationActionTitle="Midgard", WFNotificationActionBody=said(msg)),
    ]
    return workflow(actions, questions, glyph=61475, color=4292093695, share=True)


def workflow(actions, questions, glyph, color, share):
    w = {
        "WFWorkflowClientVersion": "2605.0.5",
        "WFWorkflowMinimumClientVersion": 900,
        "WFWorkflowMinimumClientVersionString": "900",
        "WFWorkflowIcon": {"WFWorkflowIconGlyphNumber": glyph, "WFWorkflowIconStartColor": color},
        "WFWorkflowImportQuestions": questions,
        "WFWorkflowActions": actions,
        "WFWorkflowInputContentItemClasses": ["WFStringContentItem", "WFImageContentItem", "WFURLContentItem",
                                              "WFRichTextContentItem", "WFSafariWebPageContentItem"],
        "WFWorkflowOutputContentItemClasses": [],
        "WFWorkflowTypes": ["ActionExtension"] if share else [],
        "WFWorkflowHasShortcutInputVariables": share,
        "WFWorkflowHasOutputFallback": False,
    }
    return w


# What each action needs said, which Shortcuts takes a file without and
# then runs wrong, or stops at: a Set Variable without its input keeps
# nothing, and the rest ask to choose a value for each parameter.
REQUIRED = {
    "setvariable": ["WFVariableName", "WFInput"],
    "getvalueforkey": ["WFInput", "WFGetDictionaryValueType", "WFDictionaryKey"],
    "detect.dictionary": ["WFInput"],
    "downloadurl": ["WFURL", "WFHTTPMethod"],
    "base64encode": ["WFInput", "WFEncodeMode"],
    "setitemname": ["WFInput", "WFName"],
    "setclipboard": ["WFInput"],
    "notification": ["WFNotificationActionBody"],
    "getitemtype": ["WFInput"],
    "image.convert": ["WFInput", "WFImageFormat"],
}


def check(name, w):
    """Refuses a Shortcut with an action missing what it needs."""
    for i, a in enumerate(w["WFWorkflowActions"]):
        kind = a["WFWorkflowActionIdentifier"].removeprefix("is.workflow.actions.")
        p = a["WFWorkflowActionParameters"]
        need = list(REQUIRED.get(kind, []))
        if kind == "conditional" and p["WFControlFlowMode"] == 0:
            need += ["WFInput", "WFCondition"] + (["WFConditionalActionString"] if p["WFCondition"] in (4, 5, 8, 9, 99, 999) else [])
        missing = [k for k in need if k not in p]
        if missing:
            raise SystemExit("%s: action %d (%s) lacks %s" % (name, i, kind, ", ".join(missing)))


def main():
    os.makedirs(OUT, exist_ok=True)
    for name, w in [("Get from Midgard", get_shortcut()), ("Send to Midgard", send_shortcut())]:
        check(name, w)
        with tempfile.TemporaryDirectory() as tmp:
            raw = os.path.join(tmp, name + ".shortcut")
            with open(raw, "wb") as f:
                plistlib.dump(w, f, fmt=plistlib.FMT_BINARY)
            out = os.path.join(OUT, name.lower().replace(" ", "-") + ".shortcut")
            subprocess.run(["shortcuts", "sign", "--mode", "anyone", "--input", raw, "--output", out], check=True)
            print(out)


if __name__ == "__main__":
    main()
