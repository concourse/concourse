module Pipeline.PinMenu.PinMenu exposing
    ( TableRow(..)
    , View
    , pinMenu
    , viewPinMenu
    )

import Colors
import Concourse
import Dict
import HoverState
import Html exposing (Html)
import Html.Attributes exposing (id, style)
import Html.Events exposing (onClick, onMouseEnter, onMouseLeave)
import Message.Message exposing (DomID(..), Message(..))
import Pipeline.PinMenu.Styles as Styles
import Pipeline.PinMenu.Views as Views
import Routes


type alias View =
    { hoverable : Bool
    , background : Views.Background
    , iconStyle : Views.Brightness
    , badge : Maybe Badge
    , dropdown : Maybe Dropdown
    }


type alias Dropdown =
    { background : String
    , position : Views.Position
    , paddingPx : Int
    , items : List DropdownItem
    }


type alias DropdownItem =
    { title : Text
    , table : List TableRow
    , onClick : Message
    }


type alias Badge =
    { color : String
    , diameterPx : Int
    , position : Views.Position
    , text : String
    }


type alias Text =
    { content : String
    , fontWeight : Int
    , color : String
    }


type TableRow
    = TableRow String String


pinMenu :
    { a | hovered : HoverState.HoverState }
    ->
        { b
            | pinnedResources : List ( String, Concourse.Version )
            , pipeline : Concourse.PipelineIdentifier
        }
    -> View
pinMenu { hovered } { pinnedResources, pipeline } =
    let
        pinCount =
            List.length pinnedResources

        hasPinnedResources =
            pinCount > 0

        isHovered =
            hovered == HoverState.Hovered PinIcon
    in
    { hoverable = hasPinnedResources
    , iconStyle =
        if hasPinnedResources then
            Views.Bright

        else
            Views.Dim
    , background =
        if hasPinnedResources && isHovered then
            Views.Spotlight

        else
            Views.Dark
    , badge =
        if hasPinnedResources then
            Just
                { color = Colors.pinned
                , diameterPx = 15
                , position = Views.TopRight (Views.Px 3) (Views.Px 3)
                , text = String.fromInt pinCount
                }

        else
            Nothing
    , dropdown =
        if isHovered then
            Just
                { background = Colors.white
                , position = Views.TopRight (Views.Percent 100) (Views.Percent 0)
                , paddingPx = 10
                , items =
                    List.map
                        (\( resourceName, pinnedVersion ) ->
                            { title =
                                { content = resourceName
                                , fontWeight = 700
                                , color = Colors.frame
                                }
                            , table =
                                pinnedVersion
                                    |> Dict.toList
                                    |> List.map (\( k, v ) -> TableRow k v)
                            , onClick =
                                GoToRoute <|
                                    Routes.Resource
                                        { id =
                                            { teamName = pipeline.teamName
                                            , pipelineName = pipeline.pipelineName
                                            , resourceName = resourceName
                                            }
                                        , page = Nothing
                                        }
                            }
                        )
                        pinnedResources
                }

        else
            Nothing
    }


viewPinMenu :
    { pinnedResources : List ( String, Concourse.Version )
    , pipeline : Concourse.PipelineIdentifier
    , isPinMenuExpanded : Bool
    }
    -> Html Message
viewPinMenu ({ isPinMenuExpanded } as params) =
    pinMenu
        { hovered =
            if isPinMenuExpanded then
                HoverState.Hovered PinIcon

            else
                HoverState.NoHover
        }
        params
        |> viewView


viewView : View -> Html Message
viewView { hoverable, background, iconStyle, badge, dropdown } =
    Html.div
        (id "pin-icon" :: Styles.pinIconContainer background)
        [ Html.div
            ((if hoverable then
                [ onMouseEnter <| Hover <| Just PinIcon
                , onMouseLeave <| Hover Nothing
                ]

              else
                []
             )
                ++ Styles.pinIcon iconStyle
            )
            ([ Maybe.map viewBadge badge
             , Maybe.map viewDropdown dropdown
             ]
                |> List.filterMap identity
            )
        ]


viewBadge : Badge -> Html Message
viewBadge badge =
    Html.div
        (id "pin-badge" :: Styles.pinBadge badge)
        [ Html.div [] [ Html.text badge.text ] ]


viewDropdown : Dropdown -> Html Message
viewDropdown dropdown =
    Html.ul
        (Styles.pinIconDropdown dropdown)
        (List.map viewDropdownItem dropdown.items)


viewDropdownItem : DropdownItem -> Html Message
viewDropdownItem item =
    Html.li
        [ onClick item.onClick, style "cursor" "pointer" ]
        [ viewTitle item.title
        , Html.table [] (List.map viewTableRow item.table)
        ]


viewTitle : Text -> Html Message
viewTitle title =
    Html.div
        (Styles.title title)
        [ Html.text title.content ]


viewTableRow : TableRow -> Html Message
viewTableRow (TableRow left right) =
    Html.tr []
        [ Html.td [] [ Html.text left ]
        , Html.td [] [ Html.text right ]
        ]
