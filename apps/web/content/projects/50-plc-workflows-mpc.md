---
id: project-plc-mpc
type: project
title: plc-workflows-mpc
slug: plc-workflows-mpc
summary: Model predictive and advanced process control running as containers on site infrastructure, with the development workflow to match.
role: Sole developer
status: active
technologies: Python, MPC/APC, Docker, CI/CD, site infrastructure servers
started: 2026
repo: https://github.com/reh3376/plc-workflows-mpc
order: 50
visibility: public
---

## The problem

Advanced process control is usually a vendor appliance or a spreadsheet
of tuning constants somebody owns personally. Neither can be reviewed,
tested before it reaches the plant, or rebuilt after the person leaves.

## The approach

Instantiate MPC and APC control as self-contained Docker containers
running on site infrastructure servers, with standard development
workflows and CI/CD pipelines around them, so a control model is
developed, reviewed and promoted the way any other software is.

## Outcomes

Active. The thinking behind it is over fifteen years of building MPC
and APC models, including the layer that sat above PLC PID control in
the three-layer control system at Whiskey House.

Active, though the public repository lags the work, which has
continued outside it.
