module Pipeline.PinMenu.PinMenu exposing
    ( TableRow
    , View
    , pinMenu
    , tooltip
    , update
    , viewPinMenu
    )

import Application.Models exposing (Session)
import Colors
import Concourse
import Dict
import EffectTransformer exposing (ET)
import HoverState
import Html exposing (Html)
import Html.Attributes exposing (id, style)
import Html.Events exposing (onClick, onMouseEnter, onMouseLeave)
import Message.Effects exposing (toHtmlID)
import Message.Message exposing (DomID(..), Message(..))
import Pipeline.PinMenu.Styles as Styles
import Pipeline.PinMenu.Views as Views
import Routes
import SideBar.Styles as SS
import Tooltip
import Views.Styles


type alias Model b =
    { b
        | fetchedResources : Maybe (List Concourse.Resource)
        , pipelineLocator : Concourse.PipelineIdentifier
        , pinMenuExpanded : Bool
    }


type alias View =
    { clickable : Bool
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
        Click TopBarPinIcon ->
            ( { model | pinMenuExpanded = not model.pinMenuExpanded }
            , effects
            )

        _ ->
            ( model, effects )


tooltip : Model b -> Session -> Maybe Tooltip.Tooltip
tooltip model session =
    case session.hovered of
        HoverState.Tooltip TopBarPinIcon _ ->
            let
                pinnedResources =
                    getPinnedResources model.fetchedResources
            in
            if model.pinMenuExpanded then
                Nothing

            else
                Just
                    { body =
                        Html.text <|
                            if List.isEmpty pinnedResources then
                                "no pinned resources"

                            else
                                "view pinned resources"
                    , attachPosition = { direction = Tooltip.Bottom, alignment = Tooltip.End }
                    , arrow = Just 5
                    , containerAttrs = Nothing
                    }

        _ ->
            Nothing


pinMenu :
    { a | hovered : HoverState.HoverState }
    -> Model b
    -> View
pinMenu { hovered } model =
    let
        pinnedResources =
            getPinnedResources model.fetchedResources

        pipeline =
            model.pipelineLocator

        pinCount =
            List.length pinnedResources

        hasPinnedResources =
            pinCount > 0

        isHovered =
            hovered == HoverState.Hovered TopBarPinIcon
    in
    { clickable = hasPinnedResources
    , opacity =
        if hasPinnedResources && (isHovered || model.pinMenuExpanded) then
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
                                    Colors.pinMenuHover

                                else
                                    Colors.pinMenuBackground
                            , hoverable = True
                            , onClick =
                                GoToRoute <|
                                    Routes.Resource
                                        { id =
                                            { teamName = pipeline.teamName
                                            , pipelineName = pipeline.pipelineName
                                            , pipelineInstanceVars = pipeline.pipelineInstanceVars
                                            , resourceName = resourceName
                                            }
                                        , page = Nothing
                                        , version = Nothing
                                        , groups = []
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


getPinnedResources : Maybe (List Concourse.Resource) -> List ( String, Concourse.Version )
getPinnedResources fetchedResources =
    case fetchedResources of
        Nothing ->
            []

        Just resources ->
            resources
                |> List.filterMap
                    (\r ->
                        Maybe.map (\v -> ( r.name, v )) r.pinnedVersion
                    )


viewView : View -> Html Message
viewView view =
    Html.div
        (([ ( onMouseEnter <| Hover <| Just TopBarPinIcon, True )
          , ( onMouseLeave <| Hover Nothing, True )
          , ( onClick <| Click TopBarPinIcon, view.clickable )
          ]
            |> List.filter Tuple.second
            |> List.map Tuple.first
         )
            ++ Styles.pinIconBackground view
        )
        (Html.div
            (id (toHtmlID TopBarPinIcon) :: Styles.pinIcon view)
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
