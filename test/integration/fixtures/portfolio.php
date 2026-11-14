<?php
/**
 * Plugin Name: Integration test portfolio type
 * Description: A custom post type whose pretty address lives only in PHP (rewrite slug "work"),
 * which is what wpexporter's local mode has to ask the web server for.
 */
add_action( 'init', function () {
	register_post_type( 'portfolio', array(
		'label'        => 'Portfolio',
		'public'       => true,
		'show_in_rest' => true,
		'rewrite'      => array( 'slug' => 'work' ),
	) );
} );
