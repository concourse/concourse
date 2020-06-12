module SideBar.SideBar exposing
    ( Model
    , hamburgerMenu
    , handleCallback
    , handleDelivery
    , tooltip
    , update
    , view
    )

import Assets
import Colors
import Concourse
import EffectTransformer exposing (ET)
import HoverState
import Html exposing (Html)
import Html.Attributes exposing (href, id, style)
import Html.Events exposing (onClick, onMouseEnter, onMouseLeave)
import List.Extra
import Message.Callback exposing (Callback(..))
import Message.Effects as Effects
import Message.Message exposing (DomID(..), Message(..))
import Message.Subscription exposing (Delivery(..))
import RemoteData exposing (RemoteData(..), WebData)
import Routes
import ScreenSize exposing (ScreenSize(..))
import Set exposing (Set)
import SideBar.Styles as Styles
import SideBar.Team as Team
import SideBar.ViewOption as ViewOption
import SideBar.ViewOptionType exposing (ViewOption(..), viewOptions)
import SideBar.Views as Views
import Tooltip
import Views.Icon as Icon


type alias Model m =
    Tooltip.Model
        { m
            | expandedTeams : Set String
            , pipelines : WebData (List Concourse.Pipeline)
            , viewOption : ViewOption
            , isSideBarOpen : Bool
            , screenSize : ScreenSize.ScreenSize
        }


type alias PipelineScoped a =
    { a
        | teamName : String
        , pipelineName : String
    }


update : Message -> Model m -> ( Model m, List Effects.Effect )
update message model =
    case message of
        Click HamburgerMenu ->
            ( { model | isSideBarOpen = not model.isSideBarOpen }
            , [ Effects.SaveSideBarState <| not model.isSideBarOpen ]
            )

        Click (SideBarTeam teamName) ->
            ( { model
                | expandedTeams =
                    if Set.member teamName model.expandedTeams then
                        Set.remove teamName model.expandedTeams

                    else
                        Set.insert teamName model.expandedTeams
              }
            , []
            )

        Click (SideBarViewOption viewOption) ->
            ( { model | viewOption = viewOption }
            , []
            )

        Hover (Just (SideBarPipeline pipelineID)) ->
            ( model
            , [ Effects.GetViewportOf
                    (SideBarPipeline pipelineID)
              ]
            )

        Hover (Just (SideBarTeam teamName)) ->
            ( model
            , [ Effects.GetViewportOf
                    (SideBarTeam teamName)
              ]
            )

        _ ->
            ( model, [] )


handleCallback : Callback -> WebData (PipelineScoped a) -> ET (Model m)
handleCallback callback currentPipeline ( model, effects ) =
    case callback of
        AllPipelinesFetched (Ok pipelines) ->
            ( { model
                | pipelines = Success pipelines
                , expandedTeams =
                    case ( model.pipelines, currentPipeline ) of
                        ( NotAsked, Success { teamName } ) ->
                            Set.insert teamName model.expandedTeams

                        _ ->
                            model.expandedTeams
              }
            , effects
            )

        BuildFetched (Ok build) ->
            ( { model
                | expandedTeams =
                    case ( currentPipeline, build.job ) of
                        ( NotAsked, Just { teamName } ) ->
                            Set.insert teamName model.expandedTeams

                        _ ->
                            model.expandedTeams
              }
            , effects
            )

        _ ->
            ( model, effects )


handleDelivery : Delivery -> ET (Model m)
handleDelivery delivery ( model, effects ) =
    case delivery of
        SideBarStateReceived (Ok True) ->
            ( { model | isSideBarOpen = True }, effects )

        _ ->
            ( model, effects )


view : Model m -> Maybe (PipelineScoped a) -> Html Message
view model currentPipeline =
    if
        model.isSideBarOpen
            && not
                (RemoteData.map List.isEmpty model.pipelines
                    |> RemoteData.withDefault True
                )
            && (model.screenSize /= ScreenSize.Mobile)
    then
        Html.div
            (id "side-bar" :: Styles.sideBar)
            (viewSelector model
                :: Styles.separator
                :: viewOptionTitle model.viewOption
                :: (model.pipelines
                        |> RemoteData.withDefault []
                        |> List.filter (ViewOption.filterFn model.viewOption)
                        |> List.Extra.gatherEqualsBy .teamName
                        |> List.map
                            (\( p, ps ) ->
                                Team.team
                                    { hovered = model.hovered
                                    , pipelines = p :: ps
                                    , currentPipeline = currentPipeline
                                    }
                                    { name = p.teamName
                                    , isExpanded = Set.member p.teamName model.expandedTeams
                                    }
                                    |> Views.viewTeam
                            )
                   )
            )

    else
        Html.text ""


viewSelector : Model m -> Html Message
viewSelector model =
    Html.div
        (id "view-selector" :: Styles.viewSelector)
        ([ Html.div Styles.viewSelectorTitle [ Html.text "Select View" ] ]
            ++ (viewOptions
                    |> List.map
                        (\v ->
                            ViewOption.viewOption model v
                                |> Views.viewOption
                        )
               )
        )


viewOptionTitle : ViewOption -> Html Message
viewOptionTitle v =
    Html.div
        (id "view-option-title" :: Styles.viewOptionTitle)
        [ Html.text <|
            case v of
                ViewNonArchivedPipelines ->
                    "Active/Paused Pipelines"

                ViewArchivedPipelines ->
                    "Archived Pipelines"
        ]


tooltip : Model m -> Maybe Tooltip.Tooltip
tooltip { hovered } =
    case hovered of
        HoverState.Tooltip (SideBarTeam teamName) _ ->
            Just
                { body = Html.div Styles.tooltipBody [ Html.text teamName ]
                , attachPosition = { direction = Tooltip.Right, alignment = Tooltip.Middle 30 }
                , arrow = Just { size = 15, color = Colors.frame }
                }

        HoverState.Tooltip (SideBarPipeline pipelineID) _ ->
            Just
                { body = Html.div Styles.tooltipBody [ Html.text pipelineID.pipelineName ]
                , attachPosition = { direction = Tooltip.Right, alignment = Tooltip.Middle 30 }
                , arrow = Just { size = 15, color = Colors.frame }
                }

        _ ->
            Nothing


hamburgerMenu :
    { a
        | screenSize : ScreenSize
        , pipelines : WebData (List Concourse.Pipeline)
        , isSideBarOpen : Bool
        , hovered : HoverState.HoverState
    }
    -> Html Message
hamburgerMenu model =
    if model.screenSize == Mobile then
        Html.text ""

    else
        let
            isHamburgerClickable =
                RemoteData.map (not << List.isEmpty) model.pipelines
                    |> RemoteData.withDefault False
        in
        Html.div
            (id "hamburger-menu"
                :: Styles.hamburgerMenu
                    { isSideBarOpen = model.isSideBarOpen
                    , isClickable = isHamburgerClickable
                    }
                ++ [ onMouseEnter <| Hover <| Just HamburgerMenu
                   , onMouseLeave <| Hover Nothing
                   ]
                ++ (if isHamburgerClickable then
                        [ onClick <| Click HamburgerMenu ]

                    else
                        []
                   )
            )
            [ Icon.icon
                { sizePx = 54, image = Assets.HamburgerMenuIcon }
              <|
                (Styles.hamburgerIcon <|
                    { isHovered =
                        isHamburgerClickable
                            && HoverState.isHovered HamburgerMenu model.hovered
                    , isActive = model.isSideBarOpen
                    }
                )
            ]
