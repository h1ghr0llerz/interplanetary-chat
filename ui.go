package main

import (
    "github.com/gdamore/tcell"
    "github.com/rivo/tview"
)

type ChatUI struct {
    app   *tview.Application
    chat  *tview.TextView
    chatbox *tview.InputField

    room_name string

    messageIngress chan string
    messageEgress chan string
}

func NewChatUI(room_name string, messageIngress chan string, messageEgress chan string) *ChatUI {
    return &ChatUI {
        room_name: room_name,
        messageIngress: messageIngress,
        messageEgress: messageEgress,
    }
}

func (c *ChatUI) InputLoop() {
    for {
        msg := <-c.messageIngress
        c.chat.SetText(c.chat.GetText(true) + msg)
    }
}

func (c *ChatUI) Start() {
    c.app = tview.NewApplication()
    c.chat = tview.NewTextView().SetScrollable(true).ScrollToEnd()
    c.chat.SetBorder(true)
    c.chat.SetTitle(c.room_name)

    c.chatbox = tview.NewInputField()
    c.chatbox.SetLabel(">\t")
    c.chatbox.SetPlaceholder("Type in your message...")

    go c.InputLoop()

    c.chatbox.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
        if event.Key() == tcell.KeyEnter {
            msg := c.chatbox.GetText()
            if msg != "" {
                c.messageEgress <- msg
            }
            c.chatbox.SetText("")
        }
        return event
    })

    flex := tview.NewFlex().
            AddItem(tview.NewFlex().SetDirection(tview.FlexRow).
                AddItem(c.chat, 0, 3, false).
                AddItem(c.chatbox, 3, 1, false), 0, 2, false)

    c.chat.SetChangedFunc(func() {
        c.app.Draw()
    })

    c.chat.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
        c.app.SetFocus(c.chatbox)
        return event
    })

    if err := c.app.SetRoot(flex, true).SetFocus(c.chatbox).Run(); err != nil {
        panic(err)
    }
}
