# Set up the GitHub Pages
# Install Jekyll
sudo apt-get install ruby-full build-essential zlib1g-dev
gem install jekyll bundler

# Create a new Jekyll site
jekyll new syncsecretakv

# Build the site and make it available on a local server
cd jekyll-syncsecretakv
bundle exec jekyll serve --host=0.0.0.0

# Now you can browse to http://localhost:4000
# Stop the server by pressing Ctrl+C

# I customized the theme by changing the _config.yml file
# I also replaced the main page with index.markdown
# At the end it's 3 files, _config.yml, index.markdown and Gemfile
# Setup github actions: https://jekyllrb.com/docs/continuous-integration/github-actions/

# Install changes
bundle install

# Build the site
bundle exec jekyll build
bundle exec jekyll build --destination docs/