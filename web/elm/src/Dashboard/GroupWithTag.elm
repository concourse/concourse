module Dashboard.GroupWithTag exposing (TaggedGroup, addTag, addTagsAndSort, headerView)

import Concourse
import Dashboard.Group as Group
import Dashboard.Group.Tag as Tag
import Html
import Html.Attributes exposing (class)
import Ordering exposing (Ordering)


type alias TaggedGroup =
    { group : Group.Group, tag : Tag.Tag }


addTagsAndSort : Concourse.User -> List Group.Group -> List TaggedGroup
addTagsAndSort user =
    List.map (addTag user) >> List.sortWith ordering


addTag : Concourse.User -> Group.Group -> TaggedGroup
addTag user g =
    { group = g
    , tag = Tag.tag user g.teamName
    }


ordering : Ordering TaggedGroup
ordering =
    Ordering.byFieldWith Tag.ordering .tag
        |> Ordering.breakTiesWith (Ordering.byFieldWith Group.ordering .group)


headerView : TaggedGroup -> List (Html.Html Group.Msg)
headerView taggedGroup =
    [ Html.div [ class "dashboard-team-name" ] [ Html.text taggedGroup.group.teamName ]
    , Html.div [ class "dashboard-team-tag" ] [ Html.text <| Tag.text taggedGroup.tag ]
    ]
