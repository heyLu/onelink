# one link

one link, per week.  every week, a member of the community chooses the
most interesting thing they recently learned about, and then all the
discussion within the community is about that topic.  community members
may post relevant other links, thoughts on the topic, or other things
that might be relevant.

one link, because other models end up being overwhelming.  different
community members post the links so that a lot of different topics will
be surfaced.  and a week to allow exploring a topic without too much
pressure, but still with a time horizon that brings a focus to the
discussion.

don't like that link?  say so, or go spend time on something else for
a bit.  maybe even suggest a link (just one) to the person choosing
the next link.  but really, doing something else will help.

## Features

- topics
    - "time limited", or maybe "until next topic post"?
    - need mechanism to decide who posts next
- comments
- users, actually "community members", because that sounds more like
    something i want -- a community of people that want to learn
    about things from experts, or just throw ideas around and see
    what happens.
    - account creation, deletion
    - maybe invite-only, or at least "application-only", somewhat
        similar to xoxo, e.g. asking people what they do?

some further ideas:

- have a "cooloff" period, after which the main message of the site
    is to "relax".  alternatively, make one day in-between a
    watercooler day, where everything is ok, but the focus is on
    talking to each other, talking about what happens, what people
    are doing, etc...
- the "topic duration" should likely be customizable, maybe sometimes
    you want to talk about stuff for a week, or maybe only a few
    days, minutes, hours.  (although the lower end of the spectrum
    would be a very different experiment.)
- "topic preview", allowing the topic author (curator?) to prepare
    the topic, especially if they have more thoughts on the topic.

## Schema

- `:topic/title`, a short string summarizing the topic
- `:topic/description`, a string containing a description of the
    topic, providing a starting point for the discussion.  (and
    also implicitely a way to further "control" the discussion
    if modifying it is allowed.  maybe only additions, not complete
    changes?  not quite sure...)

    not sure if we really need this, a first comment by the author
    might be enough.
- `:topic/url`, a url with a link to the topic being discussed

    this disallows meta-discussion for now, but not really, because
    meta-discussion can take place if someone writes something about
    the topic, and then posts a link to that thing
- `:topic/posted`, the date when the topic was posted
- `:topic/comments`, references to comments about the topic
- `:topic/id`, a short, unique, randomly generated string that will
    be used as a permanent identifier for the discussion
- `:topic/author`, a reference to the person who posted the topic
- `:comment/content`, a string containing the text of the comment
- `:comment/author`, a reference to the author of the post
- `:comment/posted`, the date when the comment was posted

    maybe call this `published` or `posted-on`?  (just `posted`
    sounds as if it's a boolean flag.  but maybe then it would
    be named `posted?`, not sure what's the convention there.)
- `:comment/replies`, a reference to replies to the comment
- `:user/name`, a unique string identifying the user
- `:user/description`, a short self-description of the user
- `:user/joined`, the date the user joined the community

## Inspirations

i.e. communities that do things in an interesting way.

- [xoxo](https://xoxofest.com)
- [meatspace](https://meatspac.es)
- [big boring system](https://bigboringsystem.com)
- news aggregators  (lots of problems with those, but they
    are a motivation for trying things differently.  and
    even with all their problems, they still surface
    interesting things on a regular basis.)
- twitter lists  (curatable, often interesting links, often
    very recent (not necessarily desirable), but often things
    not seen elsewhere, in bigger communities.  filter bubble
    problem, difficult to start with if you don't know
    anyone.)
- link blogs, like the ones of [Andy Baio](http://waxy.org)
    and [Christian Neukirchen](http://chneukirchen.org/trivium).

## Concerns

- no diversity of topics
    - e.g. because the community is too small
- only self-promotion, not enough "far out" thinking
- no general interest topics
    - this is a partial objective
