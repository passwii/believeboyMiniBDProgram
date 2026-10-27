# BelieveBoy WeChat Mini Program

Open this directory in WeChat Developer Tools. The project uses native
TypeScript, WXML and WXSS.

Before testing a real environment:

1. Replace `touristappid` in `project.config.json` with the mini program AppID.
2. Set the HTTPS API and viewer origins in
   `miniprogram/utils/config.ts`.
3. Add the viewer origin to the WeChat business-domain allowlist.
4. Add the API origin to the request-domain allowlist.

The viewer page uses `web-view`, so it must be tested on a real device after
domain configuration.
