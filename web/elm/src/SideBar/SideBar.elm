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
import Html.Attributes exposing (id)
import Html.Events exposing (onClick, onMouseDown, onMouseEnter, onMouseLeave)
import List.Extra
import Message.Callback exposing (Callback(..))
import Message.Effects as Effects
import Message.Message exposing (DomID(..), Message(..), SideBarSection(..))
import Message.Subscription exposing (Delivery(..))
import RemoteData exposing (RemoteData(..), WebData)
import ScreenSize exposing (ScreenSize(..))
import Set exposing (Set)
import SideBar.State exposing (SideBarState)
import SideBar.Styles as Styles
import SideBar.Team as Team
import SideBar.Views as Views
import Tooltip
import Views.Icon as Icon


type alias Model m =
    Tooltip.Model
        { m
            | expandedTeamsInAllPipelines : Set String
            , collapsedTeamsInFavorites : Set String
            , pipelines : WebData (List Concourse.Pipeline)
            , sideBarState : SideBarState
            , draggingSideBar : Bool
            , screenSize : ScreenSize.ScreenSize
            , favoritedPipelines : Set Int
        }


type alias PipelineScoped a =
    { a
        | teamName : String
        , pipelineName : String
    }


update : Message -> Model m -> ( Model m, List Effects.Effect )
update message model =
    let
        toggle element set =
            if Set.member element set then
                Set.remove element set

            else
                Set.insert element set
    in
    case message of
        Click HamburgerMenu ->
            let
                oldState =
                    model.sideBarState

                newState =
                    { oldState | isOpen = not oldState.isOpen }
            in
            ( { model | sideBarState = newState }
            , [ Effects.SaveSideBarState newState ]
            )

        Click (SideBarTeam section teamName) ->
            case section of
                AllPipelines ->
                    ( { model
                        | expandedTeamsInAllPipelines =
                            toggle teamName model.expandedTeamsInAllPipelines
                      }
                    , []
                    )

                Favorites ->
                    ( { model
                        | collapsedTeamsInFavorites =
                            toggle teamName model.collapsedTeamsInFavorites
                      }
                    , []
                    )

        Click SideBarResizeHandle ->
            ( { model | draggingSideBar = True }, [] )

        Click (SideBarStarIcon pipelineID) ->
            let
                favoritedPipelines =
                    toggle pipelineID model.favoritedPipelines
            in
            ( { model | favoritedPipelines = favoritedPipelines }
            , [ Effects.SaveFavoritedPipelines <| favoritedPipelines ]
            )

        Hover (Just (SideBarPipeline section pipelineID)) ->
            ( model
            , [ Effects.GetViewportOf
                    (SideBarPipeline section pipelineID)
              ]
            )

        Hover (Just (SideBarTeam section teamName)) ->
            ( model
            , [ Effects.GetViewportOf
                    (SideBarTeam section teamName)
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
                , expandedTeamsInAllPipelines =
                    case ( model.pipelines, currentPipeline ) of
                        ( NotAsked, Success { teamName } ) ->
                            model.expandedTeamsInAllPipelines
                                |> Set.insert teamName

                        _ ->
                            model.expandedTeamsInAllPipelines
              }
            , effects
            )

        BuildFetched (Ok build) ->
            ( { model
                | expandedTeamsInAllPipelines =
                    case ( currentPipeline, build.job ) of
                        ( NotAsked, Just { teamName } ) ->
                            model.expandedTeamsInAllPipelines
                                |> Set.insert teamName

                        _ ->
                            model.expandedTeamsInAllPipelines
              }
            , effects
            )

        _ ->
            ( model, effects )


handleDelivery : Delivery -> ET (Model m)
handleDelivery delivery ( model, effects ) =
    case delivery of
        SideBarStateReceived (Ok state) ->
            ( { model | sideBarState = state }, effects )

        Moused pos ->
            if model.draggingSideBar then
                let
                    oldState =
                        model.sideBarState

                    newState =
                        { oldState | width = pos.x }
                in
                ( { model | sideBarState = newState }
                , effects ++ [ Effects.GetViewportOf Dashboard ]
                )

            else
                ( model, effects )

        MouseUp ->
            ( { model | draggingSideBar = False }
            , if model.draggingSideBar then
                [ Effects.SaveSideBarState model.sideBarState ]

              else
                []
            )

        FavoritedPipelinesReceived (Ok pipelines) ->
            ( { model | favoritedPipelines = pipelines }, effects )

        _ ->
            ( model, effects )


view : Model m -> Maybe (PipelineScoped a) -> Html Message
view model currentPipeline =
    if
        model.sideBarState.isOpen
            && not
                (RemoteData.map List.isEmpty model.pipelines
                    |> RemoteData.withDefault True
                )
            && (model.screenSize /= ScreenSize.Mobile)
    then
        let
            oldState =
                model.sideBarState

            newState =
                { oldState | width = clamp 100 600 oldState.width }
        in
        Html.div
            (id "side-bar" :: Styles.sideBar newState)
            (favoritedPipelinesSection model currentPipeline
                ++ allPipelinesSection model currentPipeline
                ++ [ Html.div
                        (Styles.sideBarHandle newState
                            ++ [ onMouseDown <| Click SideBarResizeHandle ]
                        )
                        []
                   ]
            )

    else
        Html.text ""


tooltip : Model m -> Maybe Tooltip.Tooltip
tooltip { hovered } =
    case hovered of
        HoverState.Tooltip (SideBarTeam _ teamName) _ ->
            Just
                { body = Html.div Styles.tooltipBody [ Html.text teamName ]
                , attachPosition =
                    { direction =
                        Tooltip.Right (Styles.tooltipArrowSize - Styles.tooltipOffset)
                    , alignment = Tooltip.Middle <| 2 * Styles.tooltipArrowSize
                    }
                , arrow = Just { size = Styles.tooltipArrowSize, color = Colors.frame }
                }

        HoverState.Tooltip (SideBarPipeline _ pipelineID) _ ->
            Just
                { body = Html.div Styles.tooltipBody [ Html.text pipelineID.pipelineName ]
                , attachPosition =
                    { direction =
                        Tooltip.Right <|
                            Styles.tooltipArrowSize
                                + (Styles.starPadding * 2)
                                + Styles.starWidth
                                - Styles.tooltipOffset
                    , alignment = Tooltip.Middle <| 2 * Styles.tooltipArrowSize
                    }
                , arrow = Just { size = Styles.tooltipArrowSize, color = Colors.frame }
                }

        _ ->
            Nothing


allPipelinesSection : Model m -> Maybe (PipelineScoped a) -> List (Html Message)
allPipelinesSection model currentPipeline =
    [ Html.div Styles.sectionHeader [ Html.text "all pipelines" ]
    , Html.div [ id "all-pipelines" ]
        (model.pipelines
            |> RemoteData.withDefault []
            |> List.Extra.gatherEqualsBy .teamName
            |> List.map
                (\( p, ps ) ->
                    Team.team
                        { hovered = model.hovered
                        , pipelines = p :: ps
                        , currentPipeline = currentPipeline
                        , favoritedPipelines = model.favoritedPipelines
                        , isFavoritesSection = False
                        }
                        { name = p.teamName
                        , isExpanded = Set.member p.teamName model.expandedTeamsInAllPipelines
                        }
                        |> Views.viewTeam
                )
        )
    ]


favoritedPipelinesSection : Model m -> Maybe (PipelineScoped a) -> List (Html Message)
favoritedPipelinesSection model currentPipeline =
    if Set.isEmpty model.favoritedPipelines then
        []

    else
        [ Html.div Styles.sectionHeader [ Html.text "favorites" ]
        , Html.div [ id "favorites" ]
            (model.pipelines
                |> RemoteData.withDefault []
                |> List.filter
                    (\fp ->
                        Set.member fp.id model.favoritedPipelines
                    )
                |> List.Extra.gatherEqualsBy .teamName
                |> List.map
                    (\( p, ps ) ->
                        Team.team
                            { hovered = model.hovered
                            , pipelines = p :: ps
                            , currentPipeline = currentPipeline
                            , favoritedPipelines = model.favoritedPipelines
                            , isFavoritesSection = True
                            }
                            { name = p.teamName
                            , isExpanded =
                                not <|
                                    Set.member p.teamName model.collapsedTeamsInFavorites
                            }
                            |> Views.viewTeam
                    )
            )
        , Styles.separator
        ]


hamburgerMenu :
    { a
        | screenSize : ScreenSize
        , pipelines : WebData (List Concourse.Pipeline)
        , sideBarState : SideBarState
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
                    { isSideBarOpen = model.sideBarState.isOpen
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
                    , isActive = model.sideBarState.isOpen
                    }
                )
            ]
