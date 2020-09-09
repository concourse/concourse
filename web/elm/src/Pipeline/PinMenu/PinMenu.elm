module Pipeline.PinMenu.PinMenu exposing
    ( TableRow
    , View
    , pinMenu
    , update
    , viewPinMenu
    )

import Colors
import Concourse
import Dict
import EffectTransformer exposing (ET)
import HoverState
import Html exposing (Html)
import Html.Attributes exposing (id, style)
import Html.Events exposing (onClick, onMouseEnter, onMouseLeave)
import Json.Decode
import Json.Encode
import Message.Message exposing (DomID(..), Message(..))
import Pipeline.PinMenu.Styles as Styles
import Pipeline.PinMenu.Views as Views
import Routes
import SideBar.Styles as SS
import Views.Styles


type alias Model b =
    { b
        | fetchedResources : Maybe Json.Encode.Value
        , pipelineLocator : Concourse.PipelineIdentifier
        , pinMenuExpanded : Bool
    }


type alias View =
    { hoverable : Bool
    , clickable : Bool
    , background : Views.Background
    , opacity : SS.Opacity
    , badge : Maybe Badge
    , dropdown : Maybe Dropdown
    }


type alias Dropdown =
    { position : Views.Position
    , items : List DropdownItem
    }


type alias DropdownItem =
    { title : Text
    , table : List TableRow
    , paddingPx : Int
    , background : String
    , onClick : Message
    , hoverable : Bool
    }


type alias Badge =
    { color : String
    , diameterPx : Int
    , position : Views.Position
    , text : String
    }


type alias Text =
    { content : String
    , fontWeight : String
    , color : String
    }


type alias TableRow =
    { left : String
    , right : String
    , color : String
    }


update : Message -> ET (Model b)
update message ( model, effects ) =
    case message of
        Click PinIcon ->
            ( { model | pinMenuExpanded = not model.pinMenuExpanded }
            , effects
            )

        _ ->
            ( model, effects )


pinMenu :
    { a | hovered : HoverState.HoverState }
    -> Model b
    -> View
pinMenu { hovered } model =
    let
        pinnedResources =
            getPinnedResources model.fetchedResources

        pinCount =
            List.length pinnedResources

        hasPinnedResources =
            pinCount > 0

        isHovered =
            hovered == HoverState.Hovered PinIcon
    in
    { hoverable = hasPinnedResources
    , clickable = hasPinnedResources
    , opacity =
        if isHovered || model.pinMenuExpanded then
            SS.Bright

        else if hasPinnedResources then
            SS.GreyedOut

        else
            SS.Dim
    , background =
        if model.pinMenuExpanded then
            Views.Light

        else
            Views.Dark
    , badge =
        if hasPinnedResources then
            Just
                { color = Colors.pinned
                , diameterPx = 15
                , position = Views.TopRight (Views.Px 10) (Views.Px 10)
                , text = String.fromInt pinCount
                }

        else
            Nothing
    , dropdown =
        if model.pinMenuExpanded then
            Just
                { position =
                    Views.TopRight (Views.Percent 100) (Views.Percent 0)
                , items =
                    List.map
                        (\( resourceName, pinnedVersion ) ->
                            { title =
                                { content = resourceName
                                , fontWeight = Views.Styles.fontWeightDefault
                                , color = Colors.text
                                }
                            , table =
                                pinnedVersion
                                    |> Dict.toList
                                    |> List.map
                                        (\( k, v ) ->
                                            { left = k
                                            , right = v
                                            , color = Colors.text
                                            }
                                        )
                            , paddingPx = 10
                            , background =
                                if
                                    hovered
                                        == HoverState.Hovered
                                            (PinMenuDropDown resourceName)
                                then
                                    Colors.sideBarActive

                                else
                                    Colors.sideBar
                            , hoverable = True
                            , onClick =
                                GoToRoute <|
                                    Routes.Resource
                                        { id =
                                            { pipelineId = model.pipelineLocator
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
    { a | hovered : HoverState.HoverState }
    -> Model b
    -> Html Message
viewPinMenu session m =
    pinMenu session m
        |> viewView


getPinnedResources : Maybe Json.Encode.Value -> List ( String, Concourse.Version )
getPinnedResources fetchedResources =
    case fetchedResources of
        Nothing ->
            []

        Just res ->
            Json.Decode.decodeValue (Json.Decode.list Concourse.decodeResource) res
                |> Result.withDefault []
                |> List.filterMap (\r -> Maybe.map (\v -> ( r.name, v )) r.pinnedVersion)


viewView : View -> Html Message
viewView view =
    Html.div
        (([ ( id "pin-icon", True )
          , ( onMouseEnter <| Hover <| Just PinIcon, view.hoverable )
          , ( onMouseLeave <| Hover Nothing, view.hoverable )
          , ( onClick <| Click PinIcon, view.clickable )
          ]
            |> List.filter Tuple.second
            |> List.map Tuple.first
         )
            ++ Styles.pinIconBackground view
        )
        (Html.div
            (Styles.pinIcon view)
            []
            :: ([ Maybe.map viewBadge view.badge
                , Maybe.map viewDropdown view.dropdown
                ]
                    |> List.filterMap identity
               )
        )


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
        (onClick item.onClick
            :: (if item.hoverable then
                    [ onMouseEnter <|
                        Hover <|
                            Just <|
                                PinMenuDropDown item.title.content
                    , onMouseLeave <| Hover Nothing
                    ]

                else
                    []
               )
            ++ Styles.pinIconDropdownItem item
        )
        [ viewTitle item.title
        , Html.table [] (List.map viewTableRow item.table)
        ]


viewTitle : Text -> Html Message
viewTitle title =
    Html.div
        (Styles.title title)
        [ Html.text title.content ]


viewTableRow : TableRow -> Html Message
viewTableRow { left, right, color } =
    Html.tr
        [ style "color" color ]
        [ Html.td [] [ Html.text left ]
        , Html.td [] [ Html.text right ]
        ]
