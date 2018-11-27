module Dashboard.GroupWithTag exposing (TaggedGroup, addTag, addTagsAndSort, headerView)

import Concourse
import Dashboard.Group as Group
import Dashboard.Group.Tag as Tag
import Html
import Html.Attributes exposing (class)
import Maybe.Extra exposing (maybeToList)
import Ordering exposing (Ordering)


type alias TaggedGroup =
    { group : Group.Group, tag : Maybe Tag.Tag }


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


headerView : TaggedGroup -> Bool -> List (Html.Html Group.Msg)
headerView taggedGroup isHd =
    [ Html.div [ class "dashboard-team-name" ] [ Html.text taggedGroup.group.teamName ]
    ]
        ++ (maybeToList <|
                Maybe.map
                    (Tag.view isHd)
                    taggedGroup.tag
           )
